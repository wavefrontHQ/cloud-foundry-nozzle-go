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

// Preloader represents a class that's capable of loading the entire list of applications
// from CF.
type Preloader interface {
	// GetAllApps loads the entire list of applications
	GetAllApps() ([]AppInfo, error)
}

// CacheSource is called when a key isn't found in the cache. It's supposed to connect to some backend data source.
type CacheSource interface {
	// GetUncached tries to look up an object in some backend source. It returns an error if not successful.
	GetUncached(key string) (*AppInfo, error)
}
