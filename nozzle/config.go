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
	Debug         bool

	Filters *Filters `ignored:"true"`
}

type filtersConfig struct {
	MetricsBlackList []string `split_words:"true"`
	MetricsWhiteList []string `split_words:"true"`

	MetricsTagBlackList TagFilter `split_words:"true"`
	MetricsTagWhiteList TagFilter `split_words:"true"`

	TagInclude []string `split_words:"true"`
	TagExclude []string `split_words:"true"`
}

var defaultEvents = []events.Envelope_EventType{
	events.Envelope_ValueMetric,
	events.Envelope_CounterEvent,
}

// ParseConfig reads users provided env variables and create a Config
func ParseConfig() (*Config, error) {
	parseIndexedVars("FILTER_METRICS_BLACK_LIST")
	parseIndexedVars("FILTER_METRICS_WHITE_LIST")
	parseIndexedVars("FILTER_METRICS_TAG_BLACK_LIST")
	parseIndexedVars("FILTER_METRICS_TAG_WHITE_LIST")

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

	f := &filtersConfig{}
	err = envconfig.Process("filter", f)
	if err != nil {
		return nil, err
	}

	wavefrontConfig.Filters = &Filters{
		MetricsBlackList:    f.MetricsBlackList,
		MetricsWhiteList:    f.MetricsWhiteList,
		MetricsTagBlackList: f.MetricsTagBlackList,
		MetricsTagWhiteList: f.MetricsTagWhiteList,
		TagInclude:          f.TagInclude,
		TagExclude:          f.TagExclude,
	}

	config := &Config{Nozzel: nozzelConfig, WaveFront: wavefrontConfig}
	return config, nil
}

func parseIndexedVars(varName string) {
	idx := 1
	for {
		v := os.Getenv(fmt.Sprintf("%s_%d", varName, idx))
		if len(v) == 0 {
			break
		}
		newV := fmt.Sprintf("%s,%s", os.Getenv(varName), v)
		os.Setenv(varName, newV)
		idx++
	}
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
