package nozzle

import (
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

// APIClient wrapper for Cloud Foundry Client
type APIClient struct {
	clientConfig *cfclient.Config
	client       *cfclient.Client
}

// AppInfo holds Cloud Foundry applications information
type AppInfo struct {
	Name  string
	Space string
	Org   string
}

// NewAPIClient crate a new ApiClient
func NewAPIClient(conf NozzleConfig) (*APIClient, error) {
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

// ListApps wrapper for client.ListApps()
func (api *APIClient) ListApps() map[string]*AppInfo {
	appsInfo := make(map[string]*AppInfo)
	apps, _ := api.client.ListApps()
	for _, app := range apps {
		space, _ := app.Space()
		org, _ := space.Org()
		// fmt.Printf("App Name: %s - guid: %s - Space.Name:%s - org.Name: %s\n", app.Name, app.Guid, space.Name, org.Name)
		appsInfo[app.Guid] = &AppInfo{Name: app.Name, Space: space.Name, Org: org.Name}
	}
	return appsInfo
}
