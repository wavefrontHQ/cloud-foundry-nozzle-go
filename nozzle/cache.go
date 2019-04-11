package nozzle

import "time"

// Cache is an abstract representation of a cache capable of holding e.g. AppInfo records. It must
// support expiration of cache items.
type Cache interface {
	// Get returns the value for the key if it exists. If it doesn't exist, (nil, false) is returned.
	Get(key string) (interface{}, bool)

	// Set puts a key-value pair into the cache. Depending on the implementation, items may be evicted
	// from the cache if it's already at capacity.
	Set(key string, value interface{}, ttl time.Duration)
}
