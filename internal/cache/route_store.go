package cache

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"netgoat.xyz/agent/internal/policy"
)

// RouteStores owns isolated response caches. A route policy change replaces
// that route's store, ensuring responses written under one policy are never
// served under a newer one.
type RouteStores struct {
	mu     sync.Mutex
	stores map[string]routeStore
}

type routeStore struct {
	signature string
	store     *Store
}

func NewRouteStores() *RouteStores {
	return &RouteStores{stores: make(map[string]routeStore)}
}

// Store returns the cache isolated to routeKey, or nil when caching is disabled.
func (r *RouteStores) Store(routeKey string, settings policy.CacheSettings) *Store {
	if r == nil || !settings.Enabled {
		return nil
	}
	if routeKey == "" {
		routeKey = "unknown"
	}
	signature := fmt.Sprintf("%d:%d:%d", settings.TTLSeconds, settings.MaxEntries, settings.MaxBodyBytes)
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.stores[routeKey]; ok && existing.signature == signature {
		return existing.store
	}
	store := NewStore(time.Duration(settings.TTLSeconds)*time.Second, settings.MaxEntries, settings.MaxBodyBytes)
	r.stores[routeKey] = routeStore{signature: signature, store: store}
	return store
}

// Key namespaces otherwise identical requests by the route that resolved
// them. This matters when path and host policies share a hostname.
func (r *RouteStores) Key(routeKey string, request *http.Request) string {
	return routeKey + "|" + CacheKey(request)
}

// Retain removes stores for routes no longer present after a configuration
// reload. The method is deliberately optional so callers may retain a
// last-known-good cache during a transient control-plane outage.
func (r *RouteStores) Retain(routeKeys map[string]struct{}) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range r.stores {
		if _, keep := routeKeys[key]; !keep {
			delete(r.stores, key)
		}
	}
}
