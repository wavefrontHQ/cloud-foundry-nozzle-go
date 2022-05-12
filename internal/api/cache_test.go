package api

import (
	"github.com/stretchr/testify/assert"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/config"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheCallsPreLoadingOnce(t *testing.T) {
	nozzleConfig := &config.NozzleConfig{}
	mockApiClient := NewMockApiClient()
	appCache := prepareAppsCache(mockApiClient, nozzleConfig)

	for atomic.LoadInt64(&mockApiClient.ListAppsCallCount) == int64(0) {
		time.Sleep(time.Duration(10 * time.Millisecond))
	}
	appCache.getApp("some-guid")
	appCache.getApp("some-guid2")
	appCache.getApp("some-guid3")
	mockApiClient.CompleteListApps()
	for len(appCache.channel) > 0 {
		time.Sleep(time.Duration(10 * time.Millisecond))
	}

	assert.Equal(t, int64(1), atomic.LoadInt64(&mockApiClient.ListAppsCallCount), "only load cache once")
}

func TestCacheDoesntDoLookupsWhilePreloading(t *testing.T) {
	nozzleConfig := &config.NozzleConfig{}
	mockApiClient := NewMockApiClient()
	appCache := prepareAppsCache(mockApiClient, nozzleConfig)

	for atomic.LoadInt64(&mockApiClient.ListAppsCallCount) == int64(0) {
		time.Sleep(time.Duration(10 * time.Millisecond))
	}
	appCache.getApp("some-guid")
	appCache.getApp("some-guid2")
	appCache.getApp("some-guid3")
	assert.Equal(t, int64(0), atomic.LoadInt64(&mockApiClient.AppByGuidCallCount), "don't do lookups during preloading")
	mockApiClient.CompleteListApps()
	for len(appCache.channel) > 0 {
		time.Sleep(time.Duration(10 * time.Millisecond))
	}

	assert.Equal(t, int64(0), atomic.LoadInt64(&mockApiClient.AppByGuidCallCount), "don't complete lookups from preloading time")
}
