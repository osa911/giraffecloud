package tunnel

import (
	"context"
	"fmt"
	"giraffecloud/internal/config"
	"net"
	"sync"
)

type Tunnel struct {
	config     *config.Config
	conn       net.Conn
	mu         sync.Mutex
	isRunning  bool
	ctx        context.Context
	cancelFunc context.CancelFunc
}

func NewTunnel(config *config.Config) *Tunnel {
	ctx, cancel := context.WithCancel(context.Background())
	return &Tunnel{
		config:     config,
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

func (t *Tunnel) Connect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isRunning {
		return fmt.Errorf("tunnel is already running")
	}

	// TODO: Implement actual connection to GiraffeCloud server
	// This will involve:
	// 1. Establishing a secure connection (TLS)
	// 2. Authenticating with the server
	// 3. Setting up the tunnel protocol
	// 4. Starting the proxy handlers

	t.isRunning = true
	return nil
}

func (t *Tunnel) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.isRunning {
		return fmt.Errorf("tunnel is not running")
	}

	t.cancelFunc()
	if t.conn != nil {
		t.conn.Close()
	}

	t.isRunning = false
	return nil
}

func (t *Tunnel) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.isRunning
}

// TODO: Implement proxy handlers for different protocols
// This will include:
// - HTTP/HTTPS proxy
// - TCP proxy
// - UDP proxy