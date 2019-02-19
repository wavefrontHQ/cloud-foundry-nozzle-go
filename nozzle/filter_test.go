package nozzle

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoFilters(t *testing.T) {
	filters := &FiltersConfig{}
	glob := NewGlobFilter(filters)

	assert.True(t, glob.Match("ok.metric.1", nil))
	assert.True(t, glob.Match("ko.metric.1", nil))
}

func TestWhiteList(t *testing.T) {
	filters := &FiltersConfig{}
	filters.MetricsWhiteList = []string{"ok.*"}
	glob := NewGlobFilter(filters)

	assert.True(t, glob.Match("ok.metric.1", nil))
	assert.False(t, glob.Match("ko.metric.1", nil))
}

func TestBlackList(t *testing.T) {
	filters := &FiltersConfig{}
	filters.MetricsBlackList = []string{"ko.*"}
	glob := NewGlobFilter(filters)

	assert.True(t, glob.Match("ok.metric.1", nil))
	assert.False(t, glob.Match("ko.metric.1", nil))
}

func TestWhiteAndBlackList(t *testing.T) {
	filters := &FiltersConfig{}
	filters.MetricsWhiteList = []string{"ok.*"}
	filters.MetricsBlackList = []string{"*.ko"}
	glob := NewGlobFilter(filters)

	assert.True(t, glob.Match("ok.metric.1", nil))
	assert.False(t, glob.Match("ok.metric.1.ko", nil))
	assert.False(t, glob.Match("foo.metric.1.ko", nil))
}

func TestTagsWhiteAndBlackList(t *testing.T) {
	filters := &FiltersConfig{}
	filters.MetricsTagWhiteList = map[string][]string{"tag1": {"ok.*"}, "tag2": {"ok.*"}}
	filters.MetricsTagBlackList = map[string][]string{"tag1": {"*.ko"}}
	glob := NewGlobFilter(filters)

	assert.False(t, glob.Match("", map[string]string{"tag1": "tururu"}))
	assert.True(t, glob.Match("", map[string]string{"tag1": "tururu", "tag2": "ok.tururu"}))
	assert.True(t, glob.Match("", map[string]string{"tag1": "ok.tururu"}))
	assert.False(t, glob.Match("", map[string]string{"tag1": "ok.tururu.ko"}))
	assert.True(t, glob.Match("", map[string]string{"tag1": "ok.tururu", "tag2": "ok.tururu.ko"}))
}
