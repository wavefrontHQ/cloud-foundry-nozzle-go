package nozzle

import (
	"code.cloudfoundry.org/go-loggregator/v8/rpc/loggregator_v2"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/stretchr/testify/assert"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/api"
	"sync/atomic"
	"testing"
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

func (nozzle *MockApiClient) ListApps() map[string]*api.AppInfo {
	atomic.AddInt64(&nozzle.ListAppsCallCount, 1)
	for atomic.LoadInt64(&nozzle.completeListApps) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}
func (nozzle *MockApiClient) AppByGuid(guid string) (cfclient.App, error) {
	atomic.AddInt64(&nozzle.AppByGuidCallCount, 1)
	return cfclient.App{}, nil
}
func (nozzle *MockApiClient) NewAppInfo(app cfclient.App) *api.AppInfo {
	atomic.AddInt64(&nozzle.NewAppInfoCallCount, 1)
	return nil
}

func (nozzle *MockApiClient) CompleteListApps() {
	atomic.AddInt64(&nozzle.completeListApps, 1)
}

func (nozzle *MockApiClient) GetApp(guid string) *api.AppInfo {
	atomic.AddInt64(&nozzle.GetAppCallCount, 1)
	return nil
}

func TestDoesntDoAppTagLookups(t *testing.T) {
	mockApiClient := NewMockApiClient()
	nozzle := &Nozzle{
		Api:                 mockApiClient,
		enableAppTagLookups: false,
	}

	tags := map[string]string{
		"origin": "rep", "deployment": "some-deployment", "job": "some-job", "source_id": "some-guid"}

	event := &loggregator_v2.Envelope{
		SourceId: "some-guid",
		Tags:     tags,
	}
	nozzle.getTags(event)
	assert.Equal(t, int64(0), atomic.LoadInt64(&mockApiClient.GetAppCallCount), "don't do GetApp tag lookups")
}
