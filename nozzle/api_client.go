package nozzle

import (
	"log"
	"time"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
	cache "github.com/patrickmn/go-cache"
)

// APIClient wrapper for Cloud Foundry Client
type APIClient struct {
	clientConfig *cfclient.Config
	client       *cfclient.Client
	appCache     *cache.Cache
	logger       *log.Logger
}

// AppInfo holds Cloud Foundry applications information
type AppInfo struct {
	Name  string
	Space string
	Org   string
}

func newAppInfo(app cfclient.App) *AppInfo {
	space, _ := app.Space()
	org, _ := space.Org()
	return &AppInfo{Name: app.Name, Space: space.Name, Org: org.Name}
}

// NewAPIClient crate a new ApiClient
func NewAPIClient(conf *NozzleConfig, logger *log.Logger) (*APIClient, error) {
	config := &cfclient.Config{
		ApiAddress:        conf.APIURL,
		Username:          conf.Username,
		Password:          conf.Password,
		SkipSslValidation: conf.SkipSSL,
	}

	client, err := cfclient.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &APIClient{
		clientConfig: config,
		client:       client,
		appCache:     cache.New(6*time.Hour, time.Hour),
		logger:       logger,
	}, nil
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
		api.logger.Fatal("[ERROR] error getting apps info: ", err)
	}
	for _, app := range apps {
		appsInfo[app.Guid] = newAppInfo(app)
	}
	return appsInfo
}

// GetApp return cached AppInfo for a guid
func (api *APIClient) GetApp(guid string) *AppInfo {
	appInfo, found := api.appCache.Get(guid)
	if !found {
		if api.appCache.ItemCount() == 0 {
			for guid, app := range api.listApps() {
				api.appCache.Set(guid, app, 0)
			}
		} else {
			app, err := api.client.AppByGuid(guid)
			if err != nil {
				api.logger.Fatal("[ERROR] error getting app info: ", err)
			}
			api.appCache.Set(guid, newAppInfo(app), 0)
		}
		appInfo, found = api.appCache.Get(guid)
	}

	if !found {
		api.logger.Fatalf("[ERROR]  app '%s' not found", guid)
	}

	return appInfo.(*AppInfo)
}
