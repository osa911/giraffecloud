package tunnel

import (
	"net"
	"sync"
	"sync/atomic"
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

// GetConnection returns a connection using round-robin distribution
func (p *TunnelConnectionPool) GetConnection() *TunnelConnection {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.connections) == 0 {
		return nil
	}

	// Use atomic round-robin to distribute load across connections
	index := atomic.AddUint64(&p.roundRobin, 1) % uint64(len(p.connections))
	return p.connections[index]
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
	httpPool   *TunnelConnectionPool  // Pool of HTTP connections for concurrency
	wsConn     *TunnelConnection      // Single WebSocket connection
	targetPort int
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
func (m *ConnectionManager) AddConnection(domain string, conn net.Conn, targetPort int, connType ConnectionType) *TunnelConnection {
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