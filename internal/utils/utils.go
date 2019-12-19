package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/rcrowley/go-metrics"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
)

// VCAPApplication holds nozzle app info
type VCAPApplication struct {
	ID   string `json:"application_id"`
	Name string `json:"application_name"`
	Idx  int    `json:"instance_index"`
}

// GetVcapApp parse the 'VCAP_APPLICATION' env variable
func GetVcapApp() (VCAPApplication, error) {
	var app VCAPApplication
	appstr := os.Getenv("VCAP_APPLICATION")
	if len(appstr) == 0 {
		return VCAPApplication{}, fmt.Errorf("VCAP_APPLICATION variable not found")
	}

	err := json.Unmarshal([]byte(appstr), &app)
	if err != nil {
		return VCAPApplication{}, err
	}
	return app, nil
}

// GetInternalTags return tags for internal metrics
func GetInternalTags() map[string]string {
	internalTags := map[string]string{
		"foundation":               os.Getenv("WAVEFRONT_FOUNDATION"),
		"firehose-subscription-id": os.Getenv("NOZZLE_FIREHOSE_SUBSCRIPTION_ID"),
	}

	app, err := GetVcapApp()
	if err == nil {
		internalTags["application_id"] = app.ID
		internalTags["application_idx"] = fmt.Sprint(app.Idx)
		internalTags["application_name"] = app.Name
	} else {
		Logger.Printf("[ERROR] %v", err)
	}
	return internalTags
}

// NewCounter creates and register internal metrics
func NewCounter(name string, tags map[string]string) metrics.Counter {
	return reporting.GetOrRegisterMetric(name, metrics.NewCounter(), tags).(metrics.Counter)
}

var Logger = log.New(os.Stdout, "[WAVEFRONT] ", 0)
var Debug = os.Getenv("WAVEFRONT_DEBUG") == "true"
