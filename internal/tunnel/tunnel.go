package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"giraffecloud/internal/config"
	"giraffecloud/internal/tunnel/handlers"
	"net"
	"sync"
)

type Tunnel struct {
	config     *config.Config
	conn       net.Conn
	handlers   map[string]handlers.Handler
	mu         sync.Mutex
	isRunning  bool
	ctx        context.Context
	cancelFunc context.CancelFunc
}

func NewTunnel(config *config.Config) *Tunnel {
	ctx, cancel := context.WithCancel(context.Background())
	t := &Tunnel{
		config:     config,
		ctx:        ctx,
		cancelFunc: cancel,
		handlers:   make(map[string]handlers.Handler),
	}

	// Initialize handlers for each endpoint
	for _, ep := range config.Endpoints {
		handler, err := handlers.NewHandler(ep.Protocol, ep.Local)
		if err != nil {
			fmt.Printf("Error creating handler for %s: %v\n", ep.Name, err)
			continue
		}
		t.handlers[ep.Name] = handler
	}

	return t
}

func (t *Tunnel) Connect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isRunning {
		return fmt.Errorf("tunnel is already running")
	}

	// Create TLS connection
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", t.config.Server.Host, t.config.Server.Port), &tls.Config{
		InsecureSkipVerify: false, // TODO: Add proper certificate verification
	})
	if err != nil {
		return fmt.Errorf("failed to establish TLS connection: %w", err)
	}

	// Convert endpoints to protocol format
	endpoints := make([]EndpointInfo, len(t.config.Endpoints))
	for i, ep := range t.config.Endpoints {
		endpoints[i] = EndpointInfo{
			Name:     ep.Name,
			Protocol: ep.Protocol,
			Local:    ep.Local,
			Remote:   ep.Remote,
		}
	}

	// Perform handshake
	if err := PerformHandshake(conn, t.config.Token, endpoints); err != nil {
		conn.Close()
		return fmt.Errorf("handshake failed: %w", err)
	}

	t.conn = conn
	t.isRunning = true

	// Start all handlers
	for name, handler := range t.handlers {
		go func(name string, h handlers.Handler) {
			if err := h.Start(); err != nil {
				fmt.Printf("Error starting handler %s: %v\n", name, err)
			}
		}(name, handler)
	}

	// Start message handling goroutine
	go t.handleMessages()

	return nil
}

func (t *Tunnel) handleMessages() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			msg, err := ReadMessage(t.conn)
			if err != nil {
				// TODO: Implement reconnection logic
				fmt.Printf("Error reading message: %v\n", err)
				return
			}

			switch msg.Type {
			case MsgTypeData:
				// TODO: Handle data messages
				fmt.Println("Received data message")
			case MsgTypeClose:
				fmt.Println("Received close message")
				return
			case MsgTypeError:
				fmt.Printf("Received error message: %s\n", string(msg.Payload))
			default:
				fmt.Printf("Unknown message type: %x\n", msg.Type)
			}
		}
	}
}

func (t *Tunnel) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.isRunning {
		return fmt.Errorf("tunnel is not running")
	}

	// Send close message
	if t.conn != nil {
		msg := &Message{
			Type: MsgTypeClose,
		}
		if err := WriteMessage(t.conn, msg); err != nil {
			fmt.Printf("Error sending close message: %v\n", err)
		}
		t.conn.Close()
	}

	t.cancelFunc()
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