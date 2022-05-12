package api

import (
	"github.com/cloudfoundry-community/go-cfclient"
	"sync/atomic"
	"time"
)

type MockApiClient struct {
	completeListApps    int64
	ListAppsCallCount   int64
	AppByGuidCallCount  int64
	NewAppInfoCallCount int64
	GetAppCallCount     int64
}

func NewMockApiClient() *MockApiClient {
	return &MockApiClient{}
}

func (api *MockApiClient) ListApps() map[string]*AppInfo {
	atomic.AddInt64(&api.ListAppsCallCount, 1)
	for atomic.LoadInt64(&api.completeListApps) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}
func (api *MockApiClient) AppByGuid(guid string) (cfclient.App, error) {
	atomic.AddInt64(&api.AppByGuidCallCount, 1)
	return cfclient.App{}, nil
}
func (api *MockApiClient) NewAppInfo(app cfclient.App) *AppInfo {
	atomic.AddInt64(&api.NewAppInfoCallCount, 1)
	return nil
}

func (api *MockApiClient) CompleteListApps() {
	atomic.AddInt64(&api.completeListApps, 1)
}

func (api *MockApiClient) GetApp(guid string) *AppInfo {
	atomic.AddInt64(&api.GetAppCallCount, 1)
	return nil
}
