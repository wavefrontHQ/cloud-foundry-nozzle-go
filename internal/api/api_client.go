package api

import (
	"net/url"
	"strings"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/config"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/utils"
)

// APIClient wrapper for Cloud Foundry Client
type APIClient struct {
	clientConfig *cfclient.Config
	client       *cfclient.Client
	appsCahce    *appsCache
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
func NewAPIClient(nozzleConfig *config.NozzleConfig) (*APIClient, error) {
	apiURL := strings.Trim(nozzleConfig.APIURL, " ")
	if !isValidURL(apiURL) {
		apiURL = "https://" + apiURL
	}

	apiConfig := &cfclient.Config{
		ApiAddress:        apiURL,
		Username:          nozzleConfig.Username,
		Password:          nozzleConfig.Password,
		SkipSslValidation: nozzleConfig.SkipSSL,
	}

	client, err := cfclient.NewClient(apiConfig)
	if err != nil {
		return nil, err
	}

	api := &APIClient{
		clientConfig: apiConfig,
		client:       client,
	}

	api.appsCahce = prepareAppsCache(api, nozzleConfig)

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
	return api.appsCahce.getApp(guid)
}

// isValidUrl tests a string to determine if it is a url or not.
func isValidURL(toTest string) bool {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return false
	}
	return true
}
