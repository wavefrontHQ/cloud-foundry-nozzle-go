package nozzle

import (
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"time"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
	cache "github.com/patrickmn/go-cache"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
)

// APIClient wrapper for Cloud Foundry Client
type APIClient struct {
	clientConfig *cfclient.Config
	client       *cfclient.Client
	appCache     *cache.Cache
	appCacheSize int
	errors       metrics.Counter
	miss         metrics.Counter
}

// AppInfo holds Cloud Foundry applications information
type AppInfo struct {
	Name  string
	Space string
	Org   string
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
	apiURL := strings.Trim(conf.APIURL, " ")
	if !isValidURL(apiURL) {
		apiURL = "https://" + apiURL
	}
	config := &cfclient.Config{
		ApiAddress:        apiURL,
		Username:          conf.Username,
		Password:          conf.Password,
		SkipSslValidation: conf.SkipSSL,
	}

	client, err := cfclient.NewClient(config)
	if err != nil {
		return nil, err
	}

	internalTags := GetInternalTags()

	api := &APIClient{
		clientConfig: config,
		client:       client,
		appCache:     cache.New(conf.AppCacheExpiration, time.Hour),
		appCacheSize: conf.AppCacheSize,
		errors:       newCounter("cache.errors", internalTags),
		miss:         newCounter("cache.miss", internalTags),
	}

	reporting.RegisterMetric("cache.size", metrics.NewFunctionalGauge(api.cacheSize), internalTags)

	time.Sleep(time.Second * time.Duration(rand.Intn(5)))
	logger.Println("Loading app info cache")
	apps := api.listApps()
	logger.Printf("found %d apps", len(apps))

	for guid, app := range apps {
		api.appCache.Set(guid, app, 0)
	}
	logger.Println("Loading app info cache Done")

	return api, nil
}

func (api *APIClient) cacheSize() int64 {
	return int64(api.appCache.ItemCount())
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

// GetApp return cached AppInfo for a guid
func (api *APIClient) GetApp(guid string) (*AppInfo, error) {
	appInfo, found := api.appCache.Get(guid)
	if found {
		return appInfo.(*AppInfo), nil
	}

	api.miss.Inc(1)

	app, err := api.client.AppByGuid(guid)
	if err != nil {
		api.errors.Inc(1)
		return nil, fmt.Errorf("error getting app info: %v", err)
	}

	appInfo = newAppInfo(app)
	api.appCache.Set(guid, newAppInfo(app), 0)

	return appInfo.(*AppInfo), nil
}

// isValidUrl tests a string to determine if it is a url or not.
func isValidURL(toTest string) bool {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return false
	}
	return true
}
