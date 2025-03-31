package handlers

import (
	"fmt"
)

// Handler interface defines the common interface for all protocol handlers
type Handler interface {
	Start() error
}

// NewHandler creates a new handler for the given protocol and local address
func NewHandler(protocol, localAddr string) (Handler, error) {
	switch protocol {
	case "http", "https":
		return NewHTTPHandler(localAddr), nil
	case "tcp":
		return NewTCPHandler(localAddr), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}