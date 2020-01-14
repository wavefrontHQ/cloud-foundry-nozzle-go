package filter_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/config"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/filter"
)

func TestNoFilters(t *testing.T) {
	filters := &filter.Filters{}
	glob := filter.NewGlobFilter(filters)

	assert.True(t, glob.Match("ok.metric.1", nil))
	assert.True(t, glob.Match("ko.metric.1", nil))
}

func TestEmptyFilterString(t *testing.T) {
	filters := &filter.Filters{}
	filters.MetricsWhiteList = []string{""}
	glob := filter.NewGlobFilter(filters)

	assert.True(t, glob.Match("ok.metric.1", nil))
	assert.True(t, glob.Match("ko.metric.1", nil))
}

func TestWhiteList(t *testing.T) {
	filters := &filter.Filters{}
	filters.MetricsWhiteList = []string{"pcf.bosh-*-forwarder*"}
	glob := filter.NewGlobFilter(filters)

	assert.True(t, glob.Match("pcf.bosh-hm-forwarder.system.cpu.user.Load", nil))
	assert.False(t, glob.Match("pcf.container.rep.memory_bytes_quota", nil))
}

func TestBlackList(t *testing.T) {
	filters := &filter.Filters{}
	filters.MetricsBlackList = []string{"ko.*"}
	glob := filter.NewGlobFilter(filters)

	assert.True(t, glob.Match("ok.metric.1", nil))
	assert.False(t, glob.Match("ko.metric.1", nil))
}

func TestWhiteAndBlackList(t *testing.T) {
	filters := &filter.Filters{}
	filters.MetricsWhiteList = []string{"ok.*"}
	filters.MetricsBlackList = []string{"*.ko"}
	glob := filter.NewGlobFilter(filters)

	assert.True(t, glob.Match("ok.metric.1", nil))
	assert.False(t, glob.Match("ok.metric.1.ko", nil))
	assert.False(t, glob.Match("foo.metric.1.ko", nil))
}

func TestTagsWhiteAndBlackList(t *testing.T) {
	filters := &filter.Filters{}
	filters.MetricsTagWhiteList = map[string][]string{"tag1": {"ok.*"}, "tag2": {"ok.*"}}
	filters.MetricsTagBlackList = map[string][]string{"tag1": {"*.ko"}}
	glob := filter.NewGlobFilter(filters)

	assert.False(t, glob.Match("", map[string]string{"tag1": "tururu"}))
	assert.True(t, glob.Match("", map[string]string{"tag1": "tururu", "tag2": "ok.tururu"}))
	assert.True(t, glob.Match("", map[string]string{"tag1": "ok.tururu"}))
	assert.False(t, glob.Match("", map[string]string{"tag1": "ok.tururu.ko"}))
	assert.True(t, glob.Match("", map[string]string{"tag1": "ok.tururu", "tag2": "ok.tururu.ko"}))
}

func TestTagInclude(t *testing.T) {
	filters := &filter.Filters{}
	filters.TagInclude = []string{"tag[0-9]"}
	glob := filter.NewGlobFilter(filters)

	tags := map[string]string{"tag1": "tururu", "tagA": "tururu", "tag": "tururu"}
	glob.Match("", tags)
	assert.Equal(t, 1, len(tags))
}

func TestTagExclude(t *testing.T) {
	filters := &filter.Filters{}
	filters.TagExclude = []string{"tag[0-9]"}
	glob := filter.NewGlobFilter(filters)

	tags := map[string]string{"tag1": "tururu", "tagA": "tururu", "tag": "tururu"}
	glob.Match("", tags)
	assert.Equal(t, 2, len(tags))
}

func TestFiltersWithSpaces(t *testing.T) {
	os.Clearenv()
	setUpFooEnv()

	os.Setenv("FILTER_METRICS_WHITE_LIST", "foo1 , foo2,foo3 ")

	config, err := config.ParseConfig()
	if err != nil {
		assert.FailNow(t, "[ERROR] Unable to build config from environment: ", err)
	}

	filter := filter.NewGlobFilter(config.Wavefront.Filters)
	assert.True(t, filter.Match("foo1", nil))
	assert.True(t, filter.Match("foo2", nil))
	assert.True(t, filter.Match("foo3", nil))
}

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
