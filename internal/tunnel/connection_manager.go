package tunnel

import (
	"net"
	"sync"
)

// ConnectionType represents the type of tunnel connection
type ConnectionType string

const (
	ConnectionTypeHTTP      ConnectionType = "http"
	ConnectionTypeWebSocket ConnectionType = "websocket"
)

// DomainConnections holds both HTTP and WebSocket connections for a domain
type DomainConnections struct {
	domain     string
	httpConn   *TunnelConnection
	wsConn     *TunnelConnection
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
	connection := NewTunnelConnection(domain, conn, targetPort)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Get or create domain connections
	domainConns := m.connections[domain]
	if domainConns == nil {
		domainConns = &DomainConnections{
			domain:     domain,
			targetPort: targetPort,
		}
		m.connections[domain] = domainConns
	}

	// Set the appropriate connection type
	domainConns.mu.Lock()
	switch connType {
	case ConnectionTypeHTTP:
		domainConns.httpConn = connection
	case ConnectionTypeWebSocket:
		domainConns.wsConn = connection
	}
	domainConns.mu.Unlock()

	return connection
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
	switch connType {
	case ConnectionTypeHTTP:
		if domainConns.httpConn != nil {
			domainConns.httpConn.Close()
			domainConns.httpConn = nil
		}
	case ConnectionTypeWebSocket:
		if domainConns.wsConn != nil {
			domainConns.wsConn.Close()
			domainConns.wsConn = nil
		}
	}

	// Remove domain if no connections remain
	if domainConns.httpConn == nil && domainConns.wsConn == nil {
		delete(m.connections, domain)
	}
	domainConns.mu.Unlock()
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
		return domainConns.httpConn
	case ConnectionTypeWebSocket:
		return domainConns.wsConn
	default:
		return nil
	}
}

// GetHTTPConnection returns the HTTP tunnel connection for a domain (backward compatibility)
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

	return domainConns.httpConn != nil || domainConns.wsConn != nil
}

// HasHTTPConnection returns true if the domain has an active HTTP tunnel
func (m *ConnectionManager) HasHTTPConnection(domain string) bool {
	return m.GetConnection(domain, ConnectionTypeHTTP) != nil
}

// HasWebSocketConnection returns true if the domain has an active WebSocket tunnel
func (m *ConnectionManager) HasWebSocketConnection(domain string) bool {
	return m.GetConnection(domain, ConnectionTypeWebSocket) != nil
}

// Close closes all connections and cleans up resources
func (m *ConnectionManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for domain, domainConns := range m.connections {
		domainConns.mu.Lock()
		if domainConns.httpConn != nil {
			domainConns.httpConn.Close()
		}
		if domainConns.wsConn != nil {
			domainConns.wsConn.Close()
		}
		domainConns.mu.Unlock()
		delete(m.connections, domain)
	}
}