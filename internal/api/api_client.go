package api

import (
	"net/url"
	"strings"
	"sync"
	"time"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
	cache "github.com/patrickmn/go-cache"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/config"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/utils"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
)

var (
	internalTags    = utils.GetInternalTags()
	appCacheExp     = 6 * time.Hour
	appCache        = cache.New(appCacheExp, time.Hour)
	appCacheErrors  = utils.NewCounter("cache.errors", internalTags)
	appCacheMiss    = utils.NewCounter("cache.miss", internalTags)
	appCacheChannel = make(chan string, 1000)
	appCacheAPI     *APIClient
	appCacheDoOnce  sync.Once
)

func init() {
	reporting.RegisterMetric("cache.size", metrics.NewFunctionalGauge(func() int64 { return int64(appCache.ItemCount()) }), internalTags)
	go func() {
		init := false
		for guid := range appCacheChannel {
			if appCacheAPI != nil {
				if !init {
					go appCacheDoOnce.Do(func() {
						utils.Logger.Println("Loading app info cache")
						apps := appCacheAPI.listApps()
						utils.Logger.Printf("found %d apps", len(apps))
						for guid, app := range apps {
							appCache.Set(guid, app, appCacheExp)
						}
						utils.Logger.Println("Loading app info cache Done")
						init = true
					})
				} else {
					appCacheMiss.Inc(1)
					app, err := appCacheAPI.client.AppByGuid(guid)
					if err != nil {
						appCacheErrors.Inc(1)
						utils.Logger.Printf("error getting app info: %v", err)
					} else {
						appCache.Set(guid, newAppInfo(app), appCacheExp)
					}
				}
			} else {
				utils.Logger.Printf("Api == null")
			}
		}
	}()
}

// APIClient wrapper for Cloud Foundry Client
type APIClient struct {
	clientConfig *cfclient.Config
	client       *cfclient.Client
	appCacheSize int
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
		if utils.Debug {
			utils.Logger.Printf("Error getting space name for app '%s'", app.Name)
		}
		return &AppInfo{Name: app.Name, Space: "not_found", Org: "not_found"}
	}
	org, err := space.Org()
	if err != nil {
		if utils.Debug {
			utils.Logger.Printf("Error getting org name for app '%s'", app.Name)
		}
		return &AppInfo{Name: app.Name, Space: space.Name, Org: "not_found"}
	}
	return &AppInfo{Name: app.Name, Space: space.Name, Org: org.Name}
}

// NewAPIClient crate a new ApiClient
func NewAPIClient(conf *config.NozzleConfig) (*APIClient, error) {
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

	api := &APIClient{
		clientConfig: config,
		client:       client,
	}

	appCacheExp = conf.AppCacheExpiration
	appCacheAPI = api

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
		utils.Logger.Fatal("[ERROR] error getting apps info: ", err)
	}
	for _, app := range apps {
		appsInfo[app.Guid] = newAppInfo(app)
	}
	return appsInfo
}

// GetApp return cached AppInfo for a guid
func (api *APIClient) GetApp(guid string) *AppInfo {
	appInfo, found := appCache.Get(guid)
	if found {
		return appInfo.(*AppInfo)
	}
	appCacheChannel <- guid
	return nil
}

// isValidUrl tests a string to determine if it is a url or not.
func isValidURL(toTest string) bool {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return false
	}
	return true
}
