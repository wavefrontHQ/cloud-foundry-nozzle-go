package nozzle

import "time"

// Cache is an abstract representation of a cache capable of holding e.g. AppInfo records. It must
// support expiration of cache items.
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
}
