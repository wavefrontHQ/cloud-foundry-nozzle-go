package nozzle

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

type PreloadedCache struct {
	apps        *RandomEvictionCache
	appCacheURL string
	mux         sync.RWMutex
}

func NewPreloadedCache(appCacheURL string) *PreloadedCache {
	p := &PreloadedCache{
		appCacheURL: appCacheURL,
	}
	p.load()
	go p.loader()
	return p
}

func (p *PreloadedCache) Get(key string) (interface{}, bool) {
	p.mux.RLock()
	defer p.mux.RUnlock()
	return p.apps.Get(key)
}

func (p *PreloadedCache) Set(key string, value interface{}, ttl time.Duration) {
	p.mux.RLock()
	defer p.mux.RUnlock()
	p.apps.Set(key, value, ttl)
}

func (p *PreloadedCache) loader() {
	t := time.Tick(5 * time.Minute)
	for {
		<-t
		p.load()
	}
}

func (p *PreloadedCache) load() {
	pres, err := http.Get(p.appCacheURL)
	if err != nil {
		log.Fatal(err)
	}

	pbody, err := ioutil.ReadAll(pres.Body)
	pres.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	var pinfo []AppInfo
	err = json.Unmarshal(pbody, &pinfo)

	// Create new cache. We allow for an extra 100 apps to come in during the refresh period
	newApps := NewRandomEvictionCache(len(pinfo) + 100)

	for _, app := range pinfo {
		newApps.Set(app.Guid, &app, 10*time.Minute)
	}

	// Atomically overwrite old cache with new
	p.mux.Lock()
	defer p.mux.Unlock()
	p.apps = newApps

	logger.Printf("[DEBUG] Preloaded %d applications", len(pinfo))
}
