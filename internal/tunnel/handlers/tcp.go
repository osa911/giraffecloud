package handlers

import (
	"fmt"
	"io"
	"net"
	"sync"
)

type TCPHandler struct {
	localAddr string
	mu        sync.Mutex
}

func NewTCPHandler(localAddr string) *TCPHandler {
	return &TCPHandler{
		localAddr: localAddr,
	}
}

func (h *TCPHandler) HandleConnection(remoteConn net.Conn) error {
	// Connect to local service
	localConn, err := net.Dial("tcp", h.localAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to local service: %w", err)
	}
	defer localConn.Close()

	// Start bidirectional copy
	errChan := make(chan error, 2)
	go func() {
		_, err := io.Copy(localConn, remoteConn)
		errChan <- err
	}()
	go func() {
		_, err := io.Copy(remoteConn, localConn)
		errChan <- err
	}()

	// Wait for either direction to finish or error
	err = <-errChan
	if err != nil {
		return fmt.Errorf("connection error: %w", err)
	}

	return nil
}

func (h *TCPHandler) Start() error {
	listener, err := net.Listen("tcp", h.localAddr)
	if err != nil {
		return fmt.Errorf("failed to start TCP listener: %w", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept connection: %w", err)
		}

		go func(conn net.Conn) {
			if err := h.HandleConnection(conn); err != nil {
				fmt.Printf("Error handling connection: %v\n", err)
			}
			conn.Close()
		}(conn)
	}
}