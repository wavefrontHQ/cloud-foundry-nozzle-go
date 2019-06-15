package nozzle

import (
	"encoding/json"
	"fmt"
	"os"
)

// VCAPAplication holds nozzle app info
type VCAPAplication struct {
	ID   string `json:"application_id"`
	Name string `json:"application_name"`
	Idx  int    `json:"instance_index"`
}

// GetVcapApp parse the 'VCAP_APPLICATION' env variable
func GetVcapApp() (VCAPAplication, error) {
	var app VCAPAplication
	appstr := os.Getenv("VCAP_APPLICATION")
	if len(appstr) == 0 {
		return VCAPAplication{}, fmt.Errorf("VCAP_APPLICATION variable not found")
	}

	err := json.Unmarshal([]byte(appstr), &app)
	if err != nil {
		return VCAPAplication{}, err
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
		logger.Printf("[ERROR] %v", err)
	}
	return internalTags
}
