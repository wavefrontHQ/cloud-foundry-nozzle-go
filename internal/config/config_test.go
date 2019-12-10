package config_test

import (
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/common"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/legacy"
)

func setUpFooEnv() {
	os.Setenv("NOZZLE_API_URL", "foo")
	os.Setenv("NOZZLE_USERNAME", "foo")
	os.Setenv("NOZZLE_PASSWORD", "foo")
	os.Setenv("NOZZLE_FIREHOSE_SUBSCRIPTION_ID", "foo")
	os.Setenv("NOZZLE_LOG_STREAM_URL", "true")

	os.Setenv("WAVEFRONT_FLUSH_INTERVAL", "1")
	os.Setenv("WAVEFRONT_PREFIX", "foo")
	os.Setenv("WAVEFRONT_FOUNDATION", "foo")
}

func TestTagFilters(t *testing.T) {
	os.Clearenv()
	setUpFooEnv()

	os.Setenv("FILTER_METRICS_TAG_BLACK_LIST", "tag1:[foo1,foo2,foo3],tag2:[foo1,foo2]")
	config, err := common.ParseConfig()
	if err != nil {
		assert.FailNow(t, "[ERROR] Unable to build config from environment: ", err)
	}

	assert.True(t, contains(config.Wavefront.Filters.MetricsTagBlackList["tag1"], "foo1"))
	assert.True(t, contains(config.Wavefront.Filters.MetricsTagBlackList["tag1"], "foo2"))
	assert.True(t, contains(config.Wavefront.Filters.MetricsTagBlackList["tag1"], "foo3"))
	assert.True(t, contains(config.Wavefront.Filters.MetricsTagBlackList["tag2"], "foo1"))
	assert.True(t, contains(config.Wavefront.Filters.MetricsTagBlackList["tag2"], "foo2"))
	assert.False(t, contains(config.Wavefront.Filters.MetricsTagBlackList["tag2"], "foo3"))

	os.Setenv("FILTER_METRICS_TAG_BLACK_LIST", "tag1:foo1,foo2,foo3,tag1:foo1")
	config, err = common.ParseConfig()
	if err != nil {
		log.Print("[OK] Unable to build config from environment: ", err)
	} else {
		assert.FailNow(t, "ParseConfig should fail")
	}
}

func TestIndexed(t *testing.T) {
	os.Clearenv()
	setUpFooEnv()
	os.Setenv("FILTER_METRICS_BLACK_LIST", "foo1,foo2,foo3")
	os.Setenv("FILTER_METRICS_BLACK_LIST_1", "foo4")
	os.Setenv("FILTER_METRICS_BLACK_LIST_5", "foo5") // ignored

	os.Setenv("FILTER_METRICS_WHITE_LIST", "foo1,foo2,foo3")

	os.Setenv("FILTER_METRICS_TAG_BLACK_LIST", "tag1:[foo1,foo2,foo3]")
	os.Setenv("FILTER_METRICS_TAG_BLACK_LIST_1", "tag4:[foo4]")
	os.Setenv("FILTER_METRICS_TAG_BLACK_LIST_2", "tag5:[foo5],tag6:[foo6]")

	os.Setenv("FILTER_METRICS_TAG_WHITE_LIST", "tag4:[foo4]")
	os.Setenv("FILTER_METRICS_TAG_WHITE_LIST_2", "tag4:foo4")   // ignored
	os.Setenv("FILTER_METRICS_TAG_WHITE_LIST_fsd", "tag4:foo4") // ignored

	config, err := common.ParseConfig()
	if err != nil {
		assert.FailNow(t, "[ERROR] Unable to build config from environment: ", err)
	}

	log.Printf("MetricsBlackList: %v", config.Wavefront.Filters.MetricsBlackList)
	assert.Equal(t, 4, len(config.Wavefront.Filters.MetricsBlackList))

	log.Printf("MetricsWhiteList: %v", config.Wavefront.Filters.MetricsWhiteList)
	assert.Equal(t, 3, len(config.Wavefront.Filters.MetricsWhiteList))

	log.Printf("MetricsTagWhiteList: %v", config.Wavefront.Filters.MetricsTagWhiteList)
	assert.Equal(t, 1, len(config.Wavefront.Filters.MetricsTagWhiteList))

	log.Printf("MetricsTagBlackList: %v", config.Wavefront.Filters.MetricsTagBlackList)
	assert.Equal(t, 4, len(config.Wavefront.Filters.MetricsTagBlackList))

	assert.True(t, contains(config.Wavefront.Filters.MetricsTagBlackList["tag1"], "foo3"))
	assert.True(t, contains(config.Wavefront.Filters.MetricsBlackList, "foo4"))
	assert.False(t, contains(config.Wavefront.Filters.MetricsBlackList, "foo5"))
}

func TestEmptyIndexed(t *testing.T) {
	os.Clearenv()
	setUpFooEnv()

	config, err := common.ParseConfig()
	if err != nil {
		assert.FailNow(t, "[ERROR] Unable to build config from environment: ", err)
	}

	log.Printf("MetricsBlackList: %v", config.Wavefront.Filters.MetricsBlackList)
	assert.Equal(t, 0, len(config.Wavefront.Filters.MetricsBlackList))

	log.Printf("MetricsWhiteList: %v", config.Wavefront.Filters.MetricsWhiteList)
	assert.Equal(t, 0, len(config.Wavefront.Filters.MetricsWhiteList))

	log.Printf("MetricsTagWhiteList: %v", config.Wavefront.Filters.MetricsTagWhiteList)
	assert.Equal(t, 0, len(config.Wavefront.Filters.MetricsTagWhiteList))

	log.Printf("MetricsTagBlackList: %v", config.Wavefront.Filters.MetricsTagBlackList)
	assert.Equal(t, 0, len(config.Wavefront.Filters.MetricsTagBlackList))
}

func TestSelectedEventsLegacy(t *testing.T) {
	selectedEvents := legacy.ParseSelectedEvents("")
	assert.Equal(t, 3, len(selectedEvents), selectedEvents)

	selectedEvents = legacy.ParseSelectedEvents("ValueMetric,CounterEvent")
	assert.Equal(t, 2, len(selectedEvents), selectedEvents)

	os.Setenv("NOZZLE_SELECTED_EVENTS", "[ValueMetric ContainerMetric]")
	selectedEvents = legacy.ParseSelectedEvents("[ValueMetric ContainerMetric]")
	assert.Equal(t, 2, len(selectedEvents), selectedEvents)

	assertPanic(t, func() { legacy.ParseSelectedEvents("[ValueMetric Contai__nerMetric]") })
}

func assertPanic(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	f()
}

func TestLegacySelector(t *testing.T) {
	os.Clearenv()
	setUpFooEnv()
	os.Setenv("NOZZLE_LEGACY", "true")
	cfg, err := common.ParseConfig()
	if err != nil {
		assert.FailNow(t, "[ERROR] Unable to build config from environment: ", err)
	}
	assert.True(t, cfg.Nozzle.Legacy)

	os.Clearenv()
	setUpFooEnv()
	cfg, err = common.ParseConfig()
	if err != nil {
		assert.FailNow(t, "[ERROR] Unable to build config from environment: ", err)
	}
	assert.False(t, cfg.Nozzle.Legacy)
}

func TestAdvancedConfig(t *testing.T) {
	os.Clearenv()
	setUpFooEnv()
	os.Setenv("ADVANCED_CONFIG", `{"value":"yes","selected_option":{"custom_wf_proxy_addr":"addr.es","custom_wf_proxy_port":1234,"filter_metrics_black_list":"Black","filter_metrics_white_list":"White","instances":3,"selected_events":["ValueMetric","ContainerMetric"]}}`)

	cfg, err := common.ParseConfig()
	if err != nil {
		assert.FailNow(t, "[ERROR] Unable to build config from environment: ", err)
	}
	assert.Equal(t, "addr.es", cfg.Wavefront.ProxyAddr)
	assert.Equal(t, 1234, cfg.Wavefront.ProxyPort)
	assert.Equal(t, "addr.es", cfg.Wavefront.ProxyAddr)
	assert.Equal(t, "addr.es", cfg.Wavefront.ProxyAddr)
	assert.Equal(t, "White", cfg.Wavefront.Filters.MetricsWhiteList[0])
	assert.Equal(t, "Black", cfg.Wavefront.Filters.MetricsBlackList[0])

	os.Clearenv()
	setUpFooEnv()
	os.Setenv("ADVANCED_CONFIG", `{"value":"no","selected_option":{}}`)

	cfg, err = common.ParseConfig()
	if err != nil {
		assert.FailNow(t, "[ERROR] Unable to build config from environment: ", err)
	}
	assert.Equal(t, "", cfg.Wavefront.ProxyAddr)
	assert.Equal(t, 0, cfg.Wavefront.ProxyPort)
}

func contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}
