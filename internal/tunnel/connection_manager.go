package tunnel

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionType represents the type of tunnel connection
type ConnectionType string

const (
	ConnectionTypeHTTP      ConnectionType = "http"
	ConnectionTypeWebSocket ConnectionType = "websocket"
)

// TunnelConnectionPool manages a pool of HTTP tunnel connections for a domain
type TunnelConnectionPool struct {
	domain      string
	targetPort  int
	connections []*TunnelConnection
	roundRobin  uint64 // Atomic counter for round-robin distribution
	mu          sync.RWMutex
}

// NewTunnelConnectionPool creates a new connection pool
func NewTunnelConnectionPool(domain string, targetPort int) *TunnelConnectionPool {
	return &TunnelConnectionPool{
		domain:      domain,
		targetPort:  targetPort,
		connections: make([]*TunnelConnection, 0),
	}
}

// AddConnection adds a connection to the pool
func (p *TunnelConnectionPool) AddConnection(conn net.Conn) *TunnelConnection {
	tunnelConn := NewTunnelConnection(p.domain, conn, p.targetPort)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.connections = append(p.connections, tunnelConn)
	return tunnelConn
}

// IsConnectionHealthy checks if a tunnel connection is still alive and responsive
func (p *TunnelConnectionPool) IsConnectionHealthy(conn *TunnelConnection) bool {
	if conn == nil || conn.GetConn() == nil {
		return false
	}

	// Set a very short read deadline to test connection
	conn.GetConn().SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	defer conn.GetConn().SetReadDeadline(time.Time{}) // Clear deadline

	// Try to read one byte (should timeout immediately if connection is alive)
	one := make([]byte, 1)
	_, err := conn.GetConn().Read(one)

	// If we get a timeout, the connection is likely alive
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// If we get EOF or other error, connection is dead
	return false
}

// CleanupDeadConnections removes dead connections from the pool
func (p *TunnelConnectionPool) CleanupDeadConnections() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	var healthyConnections []*TunnelConnection
	removedCount := 0

	for _, conn := range p.connections {
		if p.IsConnectionHealthy(conn) {
			healthyConnections = append(healthyConnections, conn)
		} else {
			conn.Close()
			removedCount++
		}
	}

	p.connections = healthyConnections
	return removedCount
}

// GetAllConnections returns a copy of all connections in the pool
func (p *TunnelConnectionPool) GetAllConnections() []*TunnelConnection {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to avoid race conditions
	connections := make([]*TunnelConnection, len(p.connections))
	copy(connections, p.connections)
	return connections
}

// GetConnection returns a connection using round-robin distribution
func (p *TunnelConnectionPool) GetConnection() *TunnelConnection {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.connections) == 0 {
		return nil
	}

	// Use atomic round-robin to distribute load across connections
	index := atomic.AddUint64(&p.roundRobin, 1) % uint64(len(p.connections))
	conn := p.connections[index]

	// Quick health check on the selected connection
	if conn.GetConn() == nil {
		// Connection is dead, trigger cleanup and try again
		go p.CleanupDeadConnections()
		return nil
	}

	return conn
}

// RemoveConnection removes a specific connection from the pool
func (p *TunnelConnectionPool) RemoveConnection(targetConn *TunnelConnection) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, conn := range p.connections {
		if conn == targetConn {
			// Close the connection
			conn.Close()
			// Remove from slice
			p.connections = append(p.connections[:i], p.connections[i+1:]...)
			break
		}
	}
}

// Size returns the number of connections in the pool
func (p *TunnelConnectionPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.connections)
}

// Close closes all connections in the pool
func (p *TunnelConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, conn := range p.connections {
		conn.Close()
	}
	p.connections = p.connections[:0]
}

// DomainConnections holds both HTTP pool and WebSocket connections for a domain
type DomainConnections struct {
	domain     string
	httpPool   *TunnelConnectionPool // Pool of HTTP connections for concurrency
	wsConn     *TunnelConnection     // Single WebSocket connection
	targetPort int
	userID     uint32
	tunnelID   uint32
	mu         sync.RWMutex
}

// ConnectionManager handles mapping between domains and active tunnel connections
type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]*DomainConnections // domain -> connections
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*DomainConnections),
	}
}

// AddConnection adds a new connection for a domain with specified type
func (m *ConnectionManager) AddConnection(domain string, conn net.Conn, targetPort int, connType ConnectionType, userID uint32, tunnelID uint32) *TunnelConnection {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get or create domain connections
	domainConns := m.connections[domain]
	if domainConns == nil {
		domainConns = &DomainConnections{
			domain:     domain,
			targetPort: targetPort,
			httpPool:   NewTunnelConnectionPool(domain, targetPort),
		}
		m.connections[domain] = domainConns
	}
	// Set owner information if not already set
	if domainConns.userID == 0 {
		domainConns.userID = userID
	}
	if domainConns.tunnelID == 0 {
		domainConns.tunnelID = tunnelID
	}

	// Set the appropriate connection type
	domainConns.mu.Lock()
	defer domainConns.mu.Unlock()

	switch connType {
	case ConnectionTypeHTTP:
		// Add to HTTP pool for concurrent handling
		return domainConns.httpPool.AddConnection(conn)
	case ConnectionTypeWebSocket:
		// Single WebSocket connection (as before)
		connection := NewTunnelConnection(domain, conn, targetPort)
		domainConns.wsConn = connection
		return connection
	}

	return nil
}

// GetDomainOwner returns the userID and tunnelID for a domain if available
func (m *ConnectionManager) GetDomainOwner(domain string) (uint32, uint32, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	domainConns := m.connections[domain]
	if domainConns == nil {
		return 0, 0, false
	}
	return domainConns.userID, domainConns.tunnelID, true
}

// RemoveConnection removes a connection for a domain with specified type
func (m *ConnectionManager) RemoveConnection(domain string, connType ConnectionType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	domainConns := m.connections[domain]
	if domainConns == nil {
		return
	}

	domainConns.mu.Lock()
	defer domainConns.mu.Unlock()

	switch connType {
	case ConnectionTypeHTTP:
		// For HTTP, we don't remove the entire pool, just let individual connections handle cleanup
		// The pool will be cleaned up when the domain is removed entirely
	case ConnectionTypeWebSocket:
		if domainConns.wsConn != nil {
			domainConns.wsConn.Close()
			domainConns.wsConn = nil
		}
	}

	// Remove domain if no connections remain
	if domainConns.httpPool.Size() == 0 && domainConns.wsConn == nil {
		domainConns.httpPool.Close()
		delete(m.connections, domain)
	}
}

// RemoveSpecificHTTPConnection removes a specific HTTP connection from the pool
func (m *ConnectionManager) RemoveSpecificHTTPConnection(domain string, targetConn *TunnelConnection) {
	m.mu.RLock()
	domainConns := m.connections[domain]
	m.mu.RUnlock()

	if domainConns == nil {
		return
	}

	domainConns.mu.Lock()
	defer domainConns.mu.Unlock()

	domainConns.httpPool.RemoveConnection(targetConn)

	// Check if we need to remove the domain entirely
	if domainConns.httpPool.Size() == 0 && domainConns.wsConn == nil {
		m.mu.Lock()
		domainConns.httpPool.Close()
		delete(m.connections, domain)
		m.mu.Unlock()
	}
}

// GetConnection returns the tunnel connection for a domain with specified type
func (m *ConnectionManager) GetConnection(domain string, connType ConnectionType) *TunnelConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domainConns := m.connections[domain]
	if domainConns == nil {
		return nil
	}

	domainConns.mu.RLock()
	defer domainConns.mu.RUnlock()

	switch connType {
	case ConnectionTypeHTTP:
		return domainConns.httpPool.GetConnection()
	case ConnectionTypeWebSocket:
		return domainConns.wsConn
	default:
		return nil
	}
}

// GetHTTPConnection returns an HTTP tunnel connection for a domain using round-robin
func (m *ConnectionManager) GetHTTPConnection(domain string) *TunnelConnection {
	return m.GetConnection(domain, ConnectionTypeHTTP)
}

// GetWebSocketConnection returns the WebSocket tunnel connection for a domain
func (m *ConnectionManager) GetWebSocketConnection(domain string) *TunnelConnection {
	return m.GetConnection(domain, ConnectionTypeWebSocket)
}

// HasDomain returns true if the domain has any active tunnel
func (m *ConnectionManager) HasDomain(domain string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domainConns := m.connections[domain]
	if domainConns == nil {
		return false
	}

	domainConns.mu.RLock()
	defer domainConns.mu.RUnlock()

	return domainConns.httpPool.Size() > 0 || domainConns.wsConn != nil
}

// HasHTTPConnection returns true if the domain has active HTTP tunnels
func (m *ConnectionManager) HasHTTPConnection(domain string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domainConns := m.connections[domain]
	if domainConns == nil {
		return false
	}

	domainConns.mu.RLock()
	defer domainConns.mu.RUnlock()

	return domainConns.httpPool.Size() > 0
}

// HasWebSocketConnection returns true if the domain has an active WebSocket tunnel
func (m *ConnectionManager) HasWebSocketConnection(domain string) bool {
	return m.GetConnection(domain, ConnectionTypeWebSocket) != nil
}

// GetHTTPPoolSize returns the number of HTTP connections for a domain
func (m *ConnectionManager) GetHTTPPoolSize(domain string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domainConns := m.connections[domain]
	if domainConns == nil {
		return 0
	}

	domainConns.mu.RLock()
	defer domainConns.mu.RUnlock()

	return domainConns.httpPool.Size()
}

// CleanupDeadConnections removes dead connections from all pools
func (m *ConnectionManager) CleanupDeadConnections() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cleanupStats := make(map[string]int)

	for domain, domainConns := range m.connections {
		domainConns.mu.Lock()
		removed := domainConns.httpPool.CleanupDeadConnections()
		domainConns.mu.Unlock()

		if removed > 0 {
			cleanupStats[domain] = removed
		}
	}

	return cleanupStats
}

// GetAllHTTPConnections returns all HTTP connections for a domain
func (m *ConnectionManager) GetAllHTTPConnections(domain string) []*TunnelConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if domainConns, exists := m.connections[domain]; exists && domainConns.httpPool != nil {
		return domainConns.httpPool.GetAllConnections()
	}
	return nil
}

// Close closes all connections and cleans up resources
func (m *ConnectionManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for domain, domainConns := range m.connections {
		domainConns.mu.Lock()
		domainConns.httpPool.Close()
		if domainConns.wsConn != nil {
			domainConns.wsConn.Close()
		}
		domainConns.mu.Unlock()
		delete(m.connections, domain)
	}
}
