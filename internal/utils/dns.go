package utils

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/osa911/giraffecloud/internal/logging"
)

// LookupHostGlobal performs a DNS lookup using public resolvers (Google 8.8.8.8, Cloudflare 1.1.1.1)
// This bypasses the local system resolver to check for global propagation.
func LookupHostGlobal(domain string) ([]string, error) {
	logger := logging.GetGlobalLogger()

	// List of public resolvers to try
	resolvers := []string{
		"8.8.8.8:53", // Google
		"1.1.1.1:53", // Cloudflare
	}

	var lastErr error

	for _, resolverAddr := range resolvers {
		// Create a custom resolver that uses the specific DNS server
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: 2 * time.Second,
				}
				return d.DialContext(ctx, "udp", resolverAddr)
			},
		}

		// Perform lookup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ips, err := resolver.LookupHost(ctx, domain)
		cancel()

		if err == nil {
			logger.Info("Successfully resolved %s using %s: %v", domain, resolverAddr, ips)
			return ips, nil
		}

		logger.Warn("Failed to resolve %s using %s: %v", domain, resolverAddr, err)
		lastErr = err
	}

	return nil, fmt.Errorf("failed to resolve domain %s using public resolvers: %w", domain, lastErr)
}
