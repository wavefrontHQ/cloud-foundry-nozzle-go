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
	cache      *cache.Cache
	api        Client
	errors     metrics.Counter
	miss       metrics.Counter
	channel    chan string
	initDoOnce sync.Once
	preloading bool
}

func prepareAppsCache(api Client, conf *config.NozzleConfig) *appsCache {
	internalTags := utils.GetInternalTags()
	apps := &appsCache{
		cache:      cache.New(conf.AppCacheExpiration, time.Hour),
		errors:     utils.NewCounter("cache.errors", internalTags),
		miss:       utils.NewCounter("cache.miss", internalTags),
		channel:    make(chan string, 1000),
		preloading: true,
		api:        api,
	}
	reporting.RegisterMetric("cache.size", metrics.NewFunctionalGauge(func() int64 { return int64(apps.cache.ItemCount()) }), internalTags)
	apps.run()
	return apps
}

func (apps *appsCache) run() {

	go func() {

		utils.Logger.Println("Loading apps info cache")
		appsList := apps.api.ListApps()
		utils.Logger.Printf("Found %d apps", len(appsList))
		for guid, app := range appsList {
			apps.cache.Set(guid, app, cache.DefaultExpiration)
		}
		utils.Logger.Println("Loading apps info cache Done, ready to do app lookups")
		apps.preloading = false

		for guid := range apps.channel {
			apps.miss.Inc(1)
			app, err := apps.api.AppByGuid(guid)
			if err != nil {
				apps.errors.Inc(1)
				utils.Logger.Printf("error getting app info: %v", err)
			} else {
				apps.cache.Set(guid, apps.api.NewAppInfo(app), cache.DefaultExpiration)
			}
		}
	}()
}

func (apps *appsCache) getApp(guid string) *AppInfo {
	if apps.preloading {
		return nil
	}

	appInfo, found := apps.cache.Get(guid)
	if found {
		return appInfo.(*AppInfo)
	}

	select {
	case apps.channel <- guid:
	default:
	}

	return nil
}
