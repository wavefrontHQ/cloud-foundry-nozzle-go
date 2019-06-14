package nozzle

import (
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
	Username               string `required:"false"`
	Password               string `required:"false"`
	ClientID               string `required:"false" envconfig:"client_id"`
	ClientSecret           string `required:"false" envconfig:"client_secret"`
	FirehoseSubscriptionID string `required:"true" envconfig:"firehose_subscription_id"`
	SkipSSL                bool   `default:"false" envconfig:"skip_ssl"`

	AppCacheExpiration time.Duration `split_words:"true" default:"6h"`
	AppCacheSize       int           `split_words:"true" default:"50000"`
	PreloadAppCache    bool          `split_words:"true" default:"true"`

	AppCachePreloader string `split_words:"true" default:""`

	SelectedEvents []events.Envelope_EventType `ignored:"true"`
}

// WavefrontConfig holds specific Wavefront env variables
type WavefrontConfig struct {
	URL           string `envconfig:"URL"`
	Token         string `envconfig:"API_TOKEN"`
	ProxyAddr     string `envconfig:"PROXY_ADDR"`
	ProxyPort     int    `envconfig:"PROXY_PORT"`
	FlushInterval int    `default:"1" envconfig:"FLUSH_INTERVAL"`
	MaxBufferSize int    `default:"100000" envconfig:"MAX_BUFFER_SIZE"`
	BatchSize     int    `default:"10000" envconfig:"BATCH_SIZE"`
	Prefix        string `required:"true" envconfig:"PREFIX"`
	Foundation    string `required:"true" envconfig:"FOUNDATION"`

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

	nozzleConfig := &NozzleConfig{}
	err := envconfig.Process("nozzle", nozzleConfig)
	if err != nil {
		return nil, err
	}

	if !nozzleConfig.hasClientSecret() && !nozzleConfig.hasUserPass() {
		return nil, fmt.Errorf("'NOZZLE_USERNAME'-'NOZZLE_PASSWORD' or 'NOZZLE_CLIENT_ID'-'NOZZLE_CLIENT_SECRET' are required")
	}

	if nozzleConfig.hasClientSecret() && nozzleConfig.hasUserPass() {
		return nil, fmt.Errorf("only provide one config 'NOZZLE_USERNAME'-'NOZZLE_PASSWORD' or 'NOZZLE_CLIENT_ID'-'NOZZLE_CLIENT_SECRET'")
	}

	selectedEvents, err := parseSelectedEvents()
	if err != nil {
		return nil, err
	}
	nozzleConfig.SelectedEvents = selectedEvents

	wavefrontConfig := &WavefrontConfig{}
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

	config := &Config{Nozzle: nozzleConfig, Wavefront: wavefrontConfig}
	return config, nil
}

func (n NozzleConfig) hasUserPass() bool {
	return len(n.Username) > 0 && len(n.Password) > 0
}

func (n NozzleConfig) hasClientSecret() bool {
	return len(n.ClientID) > 0 && len(n.ClientSecret) > 0
}

// HasEventType returns true if a named event type is enabled.
func (n *NozzleConfig) HasEventType(e events.Envelope_EventType) bool {
	for _, s := range n.SelectedEvents {
		if s == e {
			return true
		}
	}
	return false
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
