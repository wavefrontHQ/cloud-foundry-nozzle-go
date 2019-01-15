package api

import (
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

type ApiClient struct {
	clientConfig *cfclient.Config
	client       *cfclient.Client
}

type AppInfo struct {
	Name  string
	Space string
	Org   string
}

func NewAPIClient(apiUrl string, username string, password string, sslSkipVerify bool) (*ApiClient, error) {
	config := &cfclient.Config{
		ApiAddress:        apiUrl,
		Username:          username,
		Password:          password,
		SkipSslValidation: sslSkipVerify,
	}

	client, err := cfclient.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &ApiClient{
		clientConfig: config,
		client:       client,
	}, nil
}

func (api *ApiClient) FetchTrafficControllerURL() string {
	return api.client.Endpoint.DopplerEndpoint
}

func (api *ApiClient) FetchAuthToken() (string, error) {
	token, err := api.client.GetToken()
	if err != nil {
		return "", err
	}
	return token, nil
}

func (api *ApiClient) ListApps() map[string]*AppInfo {
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
