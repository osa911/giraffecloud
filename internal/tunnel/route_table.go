package tunnel

import (
	"fmt"
	"sync"

	"github.com/osa911/giraffecloud/internal/logging"
	"github.com/osa911/giraffecloud/internal/tunnel/proto"
)

// Route represents a single tunnel routing target
type Route struct {
	Domain     string
	TargetHost string
	TargetPort int32
}

// Target returns the host:port string for dialing
func (r *Route) Target() string {
	return fmt.Sprintf("%s:%d", r.TargetHost, r.TargetPort)
}

// RouteTable maps domains to their local network targets
type RouteTable struct {
	mu     sync.RWMutex
	routes map[string]*Route
	logger *logging.Logger
}

// NewRouteTable creates an empty routing table
func NewRouteTable() *RouteTable {
	return &RouteTable{
		routes: make(map[string]*Route),
		logger: logging.GetGlobalLogger(),
	}
}

// Resolve finds the route for a given domain
func (rt *RouteTable) Resolve(domain string) *Route {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.routes[domain]
}

// Update replaces the entire routing table with new config
func (rt *RouteTable) Update(configs []*proto.TunnelRouteConfig) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	newRoutes := make(map[string]*Route, len(configs))
	for _, cfg := range configs {
		if cfg.IsEnabled {
			newRoutes[cfg.Domain] = &Route{
				Domain:     cfg.Domain,
				TargetHost: cfg.TargetHost,
				TargetPort: cfg.TargetPort,
			}
		}
	}
	rt.routes = newRoutes

	rt.logger.Info("Route table updated: serving %d tunnels", len(rt.routes))
	for domain, route := range rt.routes {
		rt.logger.Info("  %s -> %s", domain, route.Target())
	}
}

// Count returns the number of active routes
func (rt *RouteTable) Count() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return len(rt.routes)
}

// All returns all active routes
func (rt *RouteTable) All() []*Route {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	routes := make([]*Route, 0, len(rt.routes))
	for _, r := range rt.routes {
		routes = append(routes, r)
	}
	return routes
}
