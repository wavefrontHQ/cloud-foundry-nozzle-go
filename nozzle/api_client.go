package nozzle

import (
	"fmt"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
	metrics "github.com/rcrowley/go-metrics"
)

// APIClient wrapper for Cloud Foundry Client
type APIClient struct {
	clientConfig *cfclient.Config
	client       *cfclient.Client
	appCache     Cache
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
		appCache:     NewRandomEvictionCache(conf.AppCacheSize),
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
		logger.Fatal("[ERROR] error getting apps info: ", err)
	}
	for _, app := range apps {
		appsInfo[app.Guid] = newAppInfo(app)
	}
	return appsInfo
}

// GetApp return cached AppInfo for a guid
func (api *APIClient) GetApp(guid string) (*AppInfo, error) {
	//size := metrics.GetOrRegisterGauge("cache.size", nil)
	errors := metrics.GetOrRegisterCounter("cache.errors", nil)
	miss := metrics.GetOrRegisterCounter("cache.miss", nil)
	// size.Update(int64(api.appCache.ItemCount())) TODO! Try to make redis support this.

	appInfo, found := api.appCache.Get(guid)
	if found {
		return appInfo.(*AppInfo), nil
	}

	miss.Inc(1)

	app, err := api.client.AppByGuid(guid)
	if err != nil {
		errors.Inc(1)
		return nil, fmt.Errorf("error getting app info: %v", err)
	}

	appInfo = newAppInfo(app)
	return appInfo.(*AppInfo), nil
}
