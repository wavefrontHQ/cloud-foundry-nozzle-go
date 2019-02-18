package nozzle

import (
	"fmt"
	"os"
	"strings"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/kelseyhightower/envconfig"
)

// Config holds users provided env variables
type Config struct {
	Nozzel    *NozzelConfig
	WaveFront *WaveFrontConfig
}

// NozzelConfig holds specific PCF env variables
type NozzelConfig struct {
	APIURL                 string `required:"true" envconfig:"api_url"`
	Username               string `required:"true"`
	Password               string `required:"true"`
	FirehoseSubscriptionID string `required:"true" envconfig:"firehose_subscription_id"`
	SkipSSL                bool   `default:"false" envconfig:"skip_ssl"`

	SelectedEvents []events.Envelope_EventType `ignored:"true"`
}

// WaveFrontConfig holds specific Wavefront env variables
type WaveFrontConfig struct {
	URL           string `envconfig:"URL"`
	Token         string `envconfig:"API_TOKEN"`
	ProxyAddr     string `envconfig:"PROXY_ADDR"`
	ProxyPort     int    `envconfig:"PROXY_PORT"`
	FlushInterval int    `required:"true" envconfig:"FLUSH_INTERVAL"`
	Prefix        string `required:"true" envconfig:"PREFIX"`
	Foundation    string `required:"true" envconfig:"FOUNDATION"`
}

var defaultEvents = []events.Envelope_EventType{
	events.Envelope_ValueMetric,
	events.Envelope_CounterEvent,
}

// ParseConfig reads users provided env variables and create a Config
func ParseConfig() (*Config, error) {
	nozzelConfig := &NozzelConfig{}
	err := envconfig.Process("nozzle", nozzelConfig)
	if err != nil {
		return nil, err
	}

	selectedEvents, err := parseSelectedEvents()
	if err != nil {
		return nil, err
	}
	nozzelConfig.SelectedEvents = selectedEvents

	wavefrontConfig := &WaveFrontConfig{}
	err = envconfig.Process("wavefront", wavefrontConfig)
	if err != nil {
		return nil, err
	}

	config := &Config{Nozzel: nozzelConfig, WaveFront: wavefrontConfig}
	return config, nil
}

func parseSelectedEvents() ([]events.Envelope_EventType, error) {
	envValue := os.Getenv("NOZZLE_SELECTED_EVENTS")
	if envValue == "" {
		return defaultEvents, nil
	}

	selectedEvents := []events.Envelope_EventType{}
	for _, envValueSplit := range strings.Split(envValue, ",") {
		envValueSlitTrimmed := strings.TrimSpace(envValueSplit)
		val, found := events.Envelope_EventType_value[envValueSlitTrimmed]
		if found {
			selectedEvents = append(selectedEvents, events.Envelope_EventType(val))
		} else {
			return nil, fmt.Errorf("[%s] is not a valid event type", envValueSlitTrimmed)
		}
	}

	return selectedEvents, nil
}
