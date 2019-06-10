package nozzle

import (
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry-community/go-cfclient"
	"io/ioutil"
	"net/http"
)

// CFPreloader is a Preloader that reads directly from the CF API
type CFPreloader struct {
	client *cfclient.Client
}

// ExternalPreloader loads application info from an external source. An example is available here:
// https://github.com/influxdata/influxdb-firehose-nozzle/tree/master/app-api-example
type ExternalPreloader struct {
	url string
}

// NewCFPreloader creates a new Preloader that wraps the supplied CF client
func NewCFPreloader(client *cfclient.Client) *CFPreloader {
	return &CFPreloader{
		client: client,
	}
}

// GetAllApps loads the entire list of applications
func (c *CFPreloader) GetAllApps() ([]AppInfo, error) {
	apps, err := c.client.ListApps()
	if err != nil {
		return nil, err
	}
	ai := make([]AppInfo, len(apps))
	for i, a := range apps {
		ai[i].Guid = a.Guid
		ai[i].Name = a.Name
		ai[i].Org = a.SpaceData.Entity.OrgData.Entity.Name
		ai[i].Space = a.SpaceData.Entity.Name
	}
	return ai, nil
}

// NewExternalPreloader returns a preloader connected to an external source
func NewExternalPreloader(url string) *ExternalPreloader {
	return &ExternalPreloader{
		url: url,
	}
}

// GetAllApps loads the entire list of applications
func (e *ExternalPreloader) GetAllApps() ([]AppInfo, error) {
	pres, err := http.Get(e.url)
	if err != nil {
		return nil, err
	}

	code := pres.StatusCode
	if code/100 != 2 {
		return nil, fmt.Errorf("error getting all apps status=%s code=%d", pres.Status, pres.StatusCode)
	}

	pbody, err := ioutil.ReadAll(pres.Body)
	pres.Body.Close()
	if err != nil {
		return nil, err
	}

	var ai []AppInfo
	err = json.Unmarshal(pbody, &ai)
	return ai, nil
}

func (e *ExternalPreloader) GetUncached(key string) (*AppInfo, error) {
	url := e.url
	if url[len(url)-1] != '/' {
		url += "/"
	}
	pres, err := http.Get(url + key)
	if err != nil {
		return nil, err
	}

	code := pres.StatusCode
	if code/100 != 2 {
		return nil, fmt.Errorf("error getting appInfo status=%s code=%d", pres.Status, pres.StatusCode)
	}

	pbody, err := ioutil.ReadAll(pres.Body)
	pres.Body.Close()
	if err != nil {
		return nil, err
	}

	var ai AppInfo
	err = json.Unmarshal(pbody, &ai)
	return &ai, nil
}
