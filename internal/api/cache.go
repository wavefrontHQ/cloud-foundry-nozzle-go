package api

import (
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/config"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/utils"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
)

type appsCache struct {
	cache   *cache.Cache
	api     *APIClient
	errors  metrics.Counter
	miss    metrics.Counter
	channel chan string
}

var prepareCacheDoOnce sync.Once
var apps *appsCache
var internalTags = utils.GetInternalTags()

func prepareAppsCache(api *APIClient, conf *config.NozzleConfig) *appsCache {
	prepareCacheDoOnce.Do(func() {
		apps = &appsCache{
			cache:   cache.New(conf.AppCacheExpiration, time.Hour),
			errors:  utils.NewCounter("cache.errors", internalTags),
			miss:    utils.NewCounter("cache.miss", internalTags),
			channel: make(chan string, 1000),
		}
		reporting.RegisterMetric("cache.size", metrics.NewFunctionalGauge(func() int64 { return int64(apps.cache.ItemCount()) }), internalTags)
		apps.run()
	})
	apps.api = api
	return apps
}

func (apps *appsCache) run() {
	go func() {
		init := false
		for guid := range apps.channel {
			if !init {
				go appCacheDoOnce.Do(func() {
					utils.Logger.Println("Loading apps info cache")
					appsList := apps.api.listApps()
					utils.Logger.Printf("Found %d apps", len(appsList))
					for guid, app := range appsList {
						apps.cache.Set(guid, app, cache.DefaultExpiration)
					}
					utils.Logger.Println("Loading apps info cache Done")
					init = true
				})
			} else {
				apps.miss.Inc(1)
				app, err := apps.api.client.AppByGuid(guid)
				if err != nil {
					apps.errors.Inc(1)
					utils.Logger.Printf("error getting app info: %v", err)
				} else {
					apps.cache.Set(guid, newAppInfo(app), cache.DefaultExpiration)
				}
			}
		}
	}()
}

func (apps *appsCache) getApp(guid string) *AppInfo {
	appInfo, found := apps.cache.Get(guid)
	if found {
		return appInfo.(*AppInfo)
	}
	apps.channel <- guid
	return nil
}
