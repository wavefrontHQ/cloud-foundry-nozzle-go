package nozzle_test

import (
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/nozzle"
)

func setUpFooEnv() {
	os.Setenv("NOZZLE_API_URL", "foo")
	os.Setenv("NOZZLE_CLIENT_ID", "foo")
	os.Setenv("NOZZLE_CLIENT_SECRET", "foo")
	os.Setenv("NOZZLE_FIREHOSE_SUBSCRIPTION_ID", "foo")
	os.Setenv("NOZZLE_PRELOAD_APP_CACHE", "true")
	os.Setenv("WAVEFRONT_FLUSH_INTERVAL", "1")
	os.Setenv("WAVEFRONT_PREFIX", "foo")
	os.Setenv("WAVEFRONT_FOUNDATION", "foo")
}

func TestTagFilters(t *testing.T) {
	os.Clearenv()
	setUpFooEnv()

	os.Setenv("FILTER_METRICS_TAG_BLACK_LIST", "tag1:[foo1,foo2,foo3],tag2:[foo1,foo2]")
	config, err := nozzle.ParseConfig()
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
	config, err = nozzle.ParseConfig()
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

	config, err := nozzle.ParseConfig()
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

	config, err := nozzle.ParseConfig()
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

func contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}
