package tunnel

import (
	"sync/atomic"
	"time"
)

// ConnectionState represents the current state of a tunnel connection
type ConnectionState uint32

const (
	StateNew ConnectionState = iota
	StateHandshaking
	StateActive
	StateClosing
	StateClosed
)

// ConnectionStats tracks connection statistics
type ConnectionStats struct {
	BytesIn           uint64
	BytesOut          uint64
	RequestCount      uint64
	LastActivity      int64 // Unix timestamp
	ConnectedAt       int64
	HandshakeLatency  int64 // nanoseconds
	TotalErrors       uint32
	CurrentQueueSize  int32
	MaxQueueSize      int32
}

// ConnectionStateManager manages connection state and statistics
type ConnectionStateManager struct {
	state ConnectionState
	stats ConnectionStats
}

// NewConnectionStateManager creates a new connection state manager
func NewConnectionStateManager() *ConnectionStateManager {
	return &ConnectionStateManager{
		state: StateNew,
		stats: ConnectionStats{
			ConnectedAt: time.Now().Unix(),
		},
	}
}

// SetState atomically updates the connection state
func (m *ConnectionStateManager) SetState(state ConnectionState) {
	atomic.StoreUint32((*uint32)(&m.state), uint32(state))
}

// GetState atomically reads the current state
func (m *ConnectionStateManager) GetState() ConnectionState {
	return ConnectionState(atomic.LoadUint32((*uint32)(&m.state)))
}

// AddBytes atomically adds to the byte counters
func (m *ConnectionStateManager) AddBytes(in, out uint64) {
	atomic.AddUint64(&m.stats.BytesIn, in)
	atomic.AddUint64(&m.stats.BytesOut, out)
	atomic.StoreInt64(&m.stats.LastActivity, time.Now().Unix())
}

// IncrementRequests atomically increments the request counter
func (m *ConnectionStateManager) IncrementRequests() {
	atomic.AddUint64(&m.stats.RequestCount, 1)
}

// UpdateQueueSize updates the current queue size
func (m *ConnectionStateManager) UpdateQueueSize(size int32) {
	atomic.StoreInt32(&m.stats.CurrentQueueSize, size)
	for {
		maxSize := atomic.LoadInt32(&m.stats.MaxQueueSize)
		if size <= maxSize {
			break
		}
		if atomic.CompareAndSwapInt32(&m.stats.MaxQueueSize, maxSize, size) {
			break
		}
	}
}

// GetStats returns a copy of the current statistics
func (m *ConnectionStateManager) GetStats() ConnectionStats {
	return ConnectionStats{
		BytesIn:          atomic.LoadUint64(&m.stats.BytesIn),
		BytesOut:         atomic.LoadUint64(&m.stats.BytesOut),
		RequestCount:     atomic.LoadUint64(&m.stats.RequestCount),
		LastActivity:     atomic.LoadInt64(&m.stats.LastActivity),
		ConnectedAt:      m.stats.ConnectedAt,
		HandshakeLatency: atomic.LoadInt64(&m.stats.HandshakeLatency),
		TotalErrors:      atomic.LoadUint32(&m.stats.TotalErrors),
		CurrentQueueSize: atomic.LoadInt32(&m.stats.CurrentQueueSize),
		MaxQueueSize:     atomic.LoadInt32(&m.stats.MaxQueueSize),
	}
}