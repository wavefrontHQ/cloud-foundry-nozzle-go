package nozzle

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/kelseyhightower/envconfig"
)

// Config holds users provided env variables
type Config struct {
	Nozzle    *NozzleConfig
	Wavefront *WavefrontConfig
}

// NozzleConfig holds specific PCF env variables
type NozzleConfig struct {
	APIURL                 string `required:"true" envconfig:"api_url"`
	Username               string `required:"true"`
	Password               string `required:"true"`
	FirehoseSubscriptionID string `required:"true" envconfig:"firehose_subscription_id"`
	SkipSSL                bool   `default:"false" envconfig:"skip_ssl"`

	AppCacheExpiration time.Duration `split_words:"true" default:"6h"`
	AppCacheSize       int           `split_words:"true" default:"50000"`

	SelectedEvents []events.Envelope_EventType `ignored:"true"`

	AdvancedConfig advancedConfig `envconfig:"ADVANCED_CONFIG"`
}

// WavefrontConfig holds specific Wavefront env variables
type WavefrontConfig struct {
	URL           string `envconfig:"URL"`
	Token         string `envconfig:"API_TOKEN"`
	ProxyAddr     string `envconfig:"PROXY_ADDR"`
	ProxyPort     int    `envconfig:"PROXY_PORT"`
	FlushInterval int    `default:"5" envconfig:"FLUSH_INTERVAL"`
	MaxBufferSize int    `default:"100000" envconfig:"MAX_BUFFER_SIZE"`
	BatchSize     int    `default:"10000" envconfig:"BATCH_SIZE"`
	Prefix        string `required:"true" envconfig:"PREFIX"`
	Foundation    string `required:"true" envconfig:"FOUNDATION"`

	Filters *Filters `ignored:"true"`
}

type advancedConfig struct {
	Values struct {
		ProxyAddress     string   `json:"custom_wf_proxy_addr"`
		ProxyPort        int      `json:"custom_wf_proxy_port"`
		SelectedEvents   []string `json:"selected_events"`
		MetricsBlackList string   `json:"filter_metrics_black_list"`
		MetricsWhiteList string   `json:"filter_metrics_white_list"`
	} `json:"selected_option"`
}

// Decode json
func (ac *advancedConfig) Decode(value string) error {
	err := json.Unmarshal([]byte(value), &ac)
	return err
}

func (ac *advancedConfig) haveCustomProxy() bool {
	return ac.Values.ProxyPort > 0 && len(ac.Values.ProxyAddress) > 0
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
	events.Envelope_ContainerMetric,
}

// ParseConfig reads users provided env variables and create a Config
func ParseConfig() (*Config, error) {
	parseIndexedVars("FILTER_METRICS_BLACK_LIST")
	parseIndexedVars("FILTER_METRICS_WHITE_LIST")
	parseIndexedVars("FILTER_METRICS_TAG_BLACK_LIST")
	parseIndexedVars("FILTER_METRICS_TAG_WHITE_LIST")

	nozzleConfig := &NozzleConfig{}
	err := envconfig.Process("nozzle", nozzleConfig)
	if err != nil {
		return nil, err
	}

	if len(nozzleConfig.AdvancedConfig.Values.SelectedEvents) > 0 {
		os.Setenv("NOZZLE_SELECTED_EVENTS", strings.Join(nozzleConfig.AdvancedConfig.Values.SelectedEvents, ","))
	}
	selectedEvents, err := ParseSelectedEvents()
	if err != nil {
		return nil, err
	}
	nozzleConfig.SelectedEvents = selectedEvents

	wavefrontConfig := &WavefrontConfig{}
	err = envconfig.Process("wavefront", wavefrontConfig)
	if err != nil {
		return nil, err
	}

	if nozzleConfig.AdvancedConfig.haveCustomProxy() {
		wavefrontConfig.ProxyAddr = nozzleConfig.AdvancedConfig.Values.ProxyAddress
		wavefrontConfig.ProxyPort = nozzleConfig.AdvancedConfig.Values.ProxyPort
	}

	if len(nozzleConfig.AdvancedConfig.Values.MetricsWhiteList) > 0 {
		os.Setenv("FILTER_METRICS_WHITE_LIST", nozzleConfig.AdvancedConfig.Values.MetricsWhiteList)
	}
	if len(nozzleConfig.AdvancedConfig.Values.MetricsBlackList) > 0 {
		os.Setenv("FILTER_METRICS_BLACK_LIST", nozzleConfig.AdvancedConfig.Values.MetricsBlackList)
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

	// if len(nozzleConfig.AdvancedConfig.Values.)

	config := &Config{Nozzle: nozzleConfig, Wavefront: wavefrontConfig}
	return config, nil
}

// parseIndexedVars append the value of `varName_1, varName_2, varName_N` to `varName`.
// The index value have to be consecutive
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

// ParseSelectedEvents get the Selected Events from the env
func ParseSelectedEvents() ([]events.Envelope_EventType, error) {
	orgEnvValue := os.Getenv("NOZZLE_SELECTED_EVENTS")
	envValue := strings.Trim(orgEnvValue, "[]")
	if envValue == "" {
		return defaultEvents, nil
	}

	selectedEvents := []events.Envelope_EventType{}
	sep := " "
	if strings.Contains(envValue, ",") {
		sep = ","
	}
	for _, envValueSplit := range strings.Split(envValue, sep) {
		envValueSlitTrimmed := strings.TrimSpace(envValueSplit)
		val, found := events.Envelope_EventType_value[envValueSlitTrimmed]
		if found {
			selectedEvents = append(selectedEvents, events.Envelope_EventType(val))
		} else {
			return nil, fmt.Errorf("[%s] is not a valid event type", orgEnvValue)
		}
	}

	return selectedEvents, nil
}
