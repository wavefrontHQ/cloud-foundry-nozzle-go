package nozzle

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/cloudfoundry/sonde-go/events"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
	metrics "github.com/rcrowley/go-metrics"
)

// APIClient wrapper for Cloud Foundry Client
type APIClient struct {
	clientConfig *cfclient.Config
	client       *cfclient.Client
	appCache     Cache
	cacheSource  CacheSource
	expiration   time.Duration
}

func newAppInfo(app cfclient.App) *AppInfo {
	space, err := app.Space()
	if err != nil {
		if debug {
			logger.Printf("Error getting space name for app '%s'", app.Name)
		}
		return &AppInfo{Name: app.Name, Space: "not_found", Org: "not_found"}
	}
	org, err := space.Org()
	if err != nil {
		if debug {
			logger.Printf("Error getting org name for app '%s'", app.Name)
		}
		return &AppInfo{Name: app.Name, Space: space.Name, Org: "not_found"}
	}
	return &AppInfo{Name: app.Name, Space: space.Name, Org: org.Name}
}

// NewAPIClient crate a new ApiClient
func NewAPIClient(conf *NozzleConfig) (*APIClient, error) {
	config := &cfclient.Config{
		ApiAddress:        conf.APIURL,
		ClientID:          conf.ClientID,
		ClientSecret:      conf.ClientSecret,
		SkipSslValidation: conf.SkipSSL,
	}

	client, err := cfclient.NewClient(config)
	if err != nil {
		return nil, err
	}

	api := &APIClient{
		clientConfig: config,
		client:       client,
		expiration:   conf.AppCacheExpiration,
	}

	// The cache is only needed if we're collecting container metrics
	if conf.HasEventType(events.Envelope_ContainerMetric) {
		logger.Printf("Preloader URL is set to: %s, size is %d", conf.AppCachePreloader, conf.AppCacheSize)
		if conf.PreloadAppCache {
			if conf.AppCachePreloader != "" {
				pl := NewExternalPreloader(conf.AppCachePreloader)
				api.appCache = NewPreloadedCache(pl, conf.AppCacheSize)
				api.cacheSource = pl
			} else {
				api.appCache = NewPreloadedCache(NewCFPreloader(client), conf.AppCacheSize)
				api.cacheSource = api
			}
		} else {
			api.appCache = NewRandomEvictionCache(conf.AppCacheSize)
			api.cacheSource = api
		}
	}

	return api, nil
}

// FetchTrafficControllerURL return Doppler Endpoint URL
func (api *APIClient) FetchTrafficControllerURL() string {
	return api.client.Endpoint.DopplerEndpoint
}

// FetchAuthToken wrapper for client.GetToken()
func (api *APIClient) FetchAuthToken() (string, error) {
	token, err := api.client.GetToken()
	if err != nil {
		return "", err
	}
	return token, nil
}

func (api *APIClient) listApps() map[string]*AppInfo {
	appsInfo := make(map[string]*AppInfo)
	apps, err := api.client.ListApps()
	if err != nil {
		logger.Fatal("[ERROR] error getting apps info: ", err)
	}
	for _, app := range apps {
		appsInfo[app.Guid] = newAppInfo(app)
	}
	return appsInfo
}

func (api *APIClient) GetUncached(key string) (*AppInfo, error) {
	app, err := api.client.AppByGuid(key)
	if err != nil {
		return nil, err
	}
	return newAppInfo(app), nil
}

// GetApp return cached AppInfo for a guid
func (api *APIClient) GetApp(guid string) (*AppInfo, error) {
	//size := metrics.GetOrRegisterGauge("cache.size", nil)
	// size.Update(int64(api.appCache.ItemCount()))

	appInfo, found := api.appCache.Get(guid)
	if found {
		return appInfo.(*AppInfo), nil
	}

	miss := metrics.GetOrRegisterCounter("cache.miss", nil)
	miss.Inc(1)
	logger.Printf("[DEBUG] Cache miss for key: %s", guid)

	appInfo, err := api.cacheSource.GetUncached(guid)
	if err != nil {
		errors := metrics.GetOrRegisterCounter("cache.errors", nil)
		errors.Inc(1)
		return nil, fmt.Errorf("error getting app info: %v", err)
	}

	// Add a 25% fudge factor to the expiration to prevent all keys from expiring at the same time
	// causing a burst.
	e := api.expiration + time.Duration(rand.Int63n(int64(api.expiration/4)))
	logger.Printf("[DEBUG] Fudged expiration: %s", e)
	api.appCache.Set(guid, appInfo, e)
	return appInfo.(*AppInfo), nil
}
