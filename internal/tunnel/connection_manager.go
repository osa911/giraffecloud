package tunnel

import (
	"net"
	"sync"
)

// ConnectionManager handles mapping between domains and active tunnel connections
type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]*TunnelConnection // domain -> connection
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*TunnelConnection),
	}
}

// AddConnection adds a new connection for a domain
func (m *ConnectionManager) AddConnection(domain string, conn net.Conn, targetPort int) *TunnelConnection {
	connection := NewTunnelConnection(domain, conn, targetPort)

	m.mu.Lock()
	m.connections[domain] = connection
	m.mu.Unlock()

	return connection
}

// RemoveConnection removes a connection for a domain
func (m *ConnectionManager) RemoveConnection(domain string) {
	m.mu.Lock()
	if conn := m.connections[domain]; conn != nil {
		conn.Close() // Close connection and cleanup resources
	}
	delete(m.connections, domain)
	m.mu.Unlock()
}

// GetConnection returns the tunnel connection for a domain
func (m *ConnectionManager) GetConnection(domain string) *TunnelConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[domain]
}

// HasDomain returns true if the domain has an active tunnel
func (m *ConnectionManager) HasDomain(domain string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.connections[domain]
	return exists
}

// Close closes all connections and cleans up resources
func (m *ConnectionManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for domain, conn := range m.connections {
		conn.Close()
		delete(m.connections, domain)
	}
}