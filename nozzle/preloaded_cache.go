package nozzle

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

// PreloadedCache is a cache that gets preloaded (and reloaded) from a REST-service returning an array of json-encoded AppInfo
// structs. If can be used with a variety of custom services for obtaining application metadata. An example is available here:
// https://github.com/influxdata/influxdb-firehose-nozzle/tree/master/app-api-example
type PreloadedCache struct {
	apps        *RandomEvictionCache
	appCacheURL string
	mux         sync.RWMutex
	maxSize     int
}

// NewPreloadedCache creates a new PreloadedCache with a specified endpoint URL
func NewPreloadedCache(appCacheURL string, maxSize int) *PreloadedCache {
	p := &PreloadedCache{
		appCacheURL: appCacheURL,
		maxSize:     maxSize,
	}
	p.load()
	go p.loader()
	return p
}

// Get returns the value for the key if it exists. If it doesn't exist, (nil, false) is returned.
func (p *PreloadedCache) Get(key string) (interface{}, bool) {
	p.mux.RLock()
	defer p.mux.RUnlock()
	if p.apps == nil {
		return nil, false
	}
	return p.apps.Get(key)
}

// Set puts a key-value pair into the cache. If the cache is at capacity, some item will get evicted. The cache always
// looks for expired items to evict first before it starts evicting live data at random.
func (p *PreloadedCache) Set(key string, value interface{}, ttl time.Duration) {
	p.mux.RLock()
	defer p.mux.RUnlock()
	if p.apps != nil {
		p.apps.Set(key, value, ttl)
	}
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
		logger.Printf("[ERROR] Could not load cache. Will keep old cache")
		p.ensureCache()
		return
	}

	pbody, err := ioutil.ReadAll(pres.Body)
	pres.Body.Close()
	if err != nil {
		logger.Printf("[ERROR] Could not load cache. Will keep old cache")
		p.ensureCache()
		return
	}

	var pinfo []AppInfo
	err = json.Unmarshal(pbody, &pinfo)

	// Create new cache. We allow for an extra 100 apps to come in during the refresh period
	s := len(pinfo) + 100
	if s > p.maxSize {
		s = p.maxSize
	}
	newApps := NewRandomEvictionCache(s)

	for _, app := range pinfo {
		newApps.Set(app.Guid, &app, 10*time.Minute)
	}

	// Atomically overwrite old cache with new
	p.mux.Lock()
	defer p.mux.Unlock()
	p.apps = newApps

	logger.Printf("[DEBUG] Preloaded %d applications", len(pinfo))
}

func (p *PreloadedCache) ensureCache() {
	// Make sure we don't have an empty cache
	p.mux.Lock()
	defer p.mux.Unlock()
	if p.apps == nil {
		p.apps = NewRandomEvictionCache(p.maxSize)
	}
}
