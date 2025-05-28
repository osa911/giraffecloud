package tunnel

import (
	"sync"
)

const (
	// Buffer sizes based on HAProxy's defaults
	minBufferSize = 4 * 1024    // 4KB
	maxBufferSize = 256 * 1024  // 256KB
	defaultSize   = 16 * 1024   // 16KB
)

// BufferPool manages a pool of reusable buffers
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new buffer pool
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, defaultSize)
			},
		},
	}
}

// Get retrieves a buffer from the pool
func (p *BufferPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put returns a buffer to the pool
func (p *BufferPool) Put(buf []byte) {
	// Only return to pool if within size constraints
	if cap(buf) <= maxBufferSize {
		p.pool.Put(buf[:defaultSize])
	}
}

// GetWithSize gets a buffer of at least the specified size
func (p *BufferPool) GetWithSize(size int) []byte {
	if size <= defaultSize {
		return p.Get()
	}
	return make([]byte, size)
}