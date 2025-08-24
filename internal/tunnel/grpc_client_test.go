package tunnel

import (
	"testing"
)

func TestGRPCTunnelClient_ClientID(t *testing.T) {
	// Reset global counter for predictable test results
	globalClientCounter = 0

	// Create first client
	client1 := NewGRPCTunnelClient("localhost:4444", "test1.example.com", "token1", 8080, nil)
	if client1.GetClientID() != "grpc-client-1" {
		t.Errorf("Expected client1 ID to be 'grpc-client-1', got '%s'", client1.GetClientID())
	}

	// Create second client
	client2 := NewGRPCTunnelClient("localhost:4444", "test2.example.com", "token2", 8081, nil)
	if client2.GetClientID() != "grpc-client-2" {
		t.Errorf("Expected client2 ID to be 'grpc-client-2', got '%s'", client2.GetClientID())
	}

	// Create third client
	client3 := NewGRPCTunnelClient("localhost:4444", "test3.example.com", "token3", 8082, nil)
	if client3.GetClientID() != "grpc-client-3" {
		t.Errorf("Expected client3 ID to be 'grpc-client-3', got '%s'", client3.GetClientID())
	}

	// Verify all clients have unique IDs
	ids := map[string]bool{
		client1.GetClientID(): true,
		client2.GetClientID(): true,
		client3.GetClientID(): true,
	}

	if len(ids) != 3 {
		t.Errorf("Expected 3 unique client IDs, got %d", len(ids))
	}
}

func TestGRPCTunnelClient_ClientIDFormat(t *testing.T) {
	// Reset global counter
	globalClientCounter = 0

	client := NewGRPCTunnelClient("localhost:4444", "test.example.com", "token", 8080, nil)
	clientID := client.GetClientID()

	// Verify the format is "grpc-client-{number}"
	if len(clientID) < 12 {
		t.Errorf("Client ID too short: %s", clientID)
	}

	if clientID[:12] != "grpc-client-" {
		t.Errorf("Client ID should start with 'grpc-client-', got: %s", clientID)
	}
}
