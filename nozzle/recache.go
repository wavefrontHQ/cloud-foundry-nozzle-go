package nozzle

import (
	"container/heap"
	"math/rand"
	"sync"
	"time"
)

type cacheEntry struct {
	key        string
	value      interface{}
	index      int
	expiration int64
}

type cacheEntries []*cacheEntry

func (ce cacheEntries) Len() int { return len(ce) }

func (ce cacheEntries) Less(i, j int) bool {
	return ce[i].expiration < ce[j].expiration
}

func (ce cacheEntries) Swap(i, j int) {
	ce[i], ce[j] = ce[j], ce[i]
	ce[i].index = i
	ce[j].index = j
}

func (ce *cacheEntries) Push(x interface{}) {
	n := len(*ce)
	item := x.(*cacheEntry)
	item.index = n
	*ce = append(*ce, item)
}

func (ce *cacheEntries) Pop() interface{} {
	old := *ce
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*ce = old[0 : n-1]
	return item
}

// RandomeEvictionCache implements a cache with a random eviction policy. LRU caches suffer from very poor performance
// when data is read in a predicable sequence from an undersized cache. Since we expect the sequence of data on the nozzle
// to be predicable, this type of cache probably performs better.
type RandomEvictionCache struct {
	keys    map[string]*cacheEntry
	entries cacheEntries
	size    int
	mux     sync.RWMutex
}

func NewRandomEvictionCache(size int) *RandomEvictionCache {
	c := RandomEvictionCache{
		keys:    make(map[string]*cacheEntry),
		entries: make([]*cacheEntry, 0, size),
		size:    size,
	}
	return &c
}

func (r *RandomEvictionCache) Get(key string) (interface{}, bool) {
	r.mux.RLock()
	defer r.mux.RUnlock()
	if ce, ok := r.keys[key]; ok {
		if ce.expiration < time.Now().UnixNano() {
			return nil, false
		}
		return ce.value, true
	}
	return nil, false
}

func (r *RandomEvictionCache) Set(key string, value interface{}, ttl time.Duration) {
	r.mux.Lock()
	defer r.mux.Unlock()

	expiration := time.Now().Add(ttl).UnixNano()
	ce := &cacheEntry{
		expiration: expiration,
		index:      0,
		key:        key,
		value:      value,
	}

	// At capacity?
	if len(r.entries) >= r.size {
		// Can we get rid of an expired item?
		var i int
		if r.entries[0].expiration < time.Now().UnixNano() {
			i = 0
		} else {
			// Pick a random item to overwrite
			i = rand.Intn(len(r.entries))
		}
		delete(r.keys, r.entries[i].key)
		ce.index = i
		r.entries[i] = ce
		r.keys[key] = ce
		// Rebalance heap
		heap.Fix(&r.entries, i)
	} else {
		// Not yet at capacity. Just push it.

		heap.Push(&r.entries, ce)
		r.keys[key] = ce
	}
}
