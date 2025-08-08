package tunnel

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// ConnectionPool manages a pool of connections to local services
type ConnectionPool struct {
	host        string
	port        int
	maxSize     int
	connections chan net.Conn
	mu          sync.RWMutex
	closed      bool
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(host string, port int, maxSize int) *ConnectionPool {
	if maxSize <= 0 {
		maxSize = 10 // Default pool size
	}

	return &ConnectionPool{
		host:        host,
		port:        port,
		maxSize:     maxSize,
		connections: make(chan net.Conn, maxSize),
		closed:      false,
	}
}

// Get retrieves a connection from the pool or creates a new one
func (p *ConnectionPool) Get() (net.Conn, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, fmt.Errorf("connection pool is closed")
	}
	p.mu.RUnlock()

	// Try to get a connection from the pool
	select {
	case conn := <-p.connections:
		// Test if connection is still alive
		if p.isConnectionAlive(conn) {
			return conn, nil
		}
		// Connection is dead, close it and create a new one
		conn.Close()
		return p.createConnection()
	default:
		// No connection available, create a new one
		return p.createConnection()
	}
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(conn net.Conn) {
	if conn == nil {
		return
	}

	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		conn.Close()
		return
	}
	p.mu.RUnlock()

	// Test if connection is still alive before returning to pool
	if !p.isConnectionAlive(conn) {
		conn.Close()
		return
	}

	// Try to put connection back in pool
	select {
	case p.connections <- conn:
		// Successfully returned to pool
	default:
		// Pool is full, close the connection
		conn.Close()
	}
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}
	p.closed = true

	// Close all connections in the pool
	close(p.connections)
	for conn := range p.connections {
		conn.Close()
	}
}

// createConnection creates a new connection to the local service
func (p *ConnectionPool) createConnection() (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second, // Keep connections alive longer
	}

	addr := fmt.Sprintf("%s:%d", p.host, p.port)
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	// Set TCP keepalive for better connection management
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	return conn, nil
}

// isConnectionAlive tests if a connection is still usable
func (p *ConnectionPool) isConnectionAlive(conn net.Conn) bool {
	// Set a very short deadline for the test
	conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	defer conn.SetReadDeadline(time.Time{}) // Clear deadline

	// Try to read one byte (this should timeout immediately if connection is alive)
	one := make([]byte, 1)
	_, err := conn.Read(one)

	// If we get a timeout, the connection is likely alive
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// If we get EOF or other error, connection is dead
	return false
}

// Stats returns pool statistics
func (p *ConnectionPool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"available_connections": len(p.connections),
		"max_size":              p.maxSize,
		"closed":                p.closed,
		"target":                fmt.Sprintf("%s:%d", p.host, p.port),
	}
}
