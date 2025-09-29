package tunnel

import (
	"testing"
)

func TestGRPCTunnelClient_ClientID(t *testing.T) {
	// Reset global counter and process stable ID for predictable test results
	globalClientCounter = 0
	processStableClientID = ""

	// Create first client - this should set the process stable ID
	client1 := NewGRPCTunnelClient("localhost:4444", "test1.example.com", "token1", 8080, nil)
	expectedID := "grpc-client-1"
	if client1.GetClientID() != expectedID {
		t.Errorf("Expected client1 ID to be '%s', got '%s'", expectedID, client1.GetClientID())
	}

	// Create second client - should reuse the same process stable ID
	client2 := NewGRPCTunnelClient("localhost:4444", "test2.example.com", "token2", 8081, nil)
	if client2.GetClientID() != expectedID {
		t.Errorf("Expected client2 ID to be '%s' (process stable), got '%s'", expectedID, client2.GetClientID())
	}

	// Create third client - should also reuse the same process stable ID
	client3 := NewGRPCTunnelClient("localhost:4444", "test3.example.com", "token3", 8082, nil)
	if client3.GetClientID() != expectedID {
		t.Errorf("Expected client3 ID to be '%s' (process stable), got '%s'", expectedID, client3.GetClientID())
	}

	// Verify all clients have the SAME ID (process stable behavior)
	if client1.GetClientID() != client2.GetClientID() || client2.GetClientID() != client3.GetClientID() {
		t.Errorf("Expected all clients to have the same process-stable ID, got: %s, %s, %s",
			client1.GetClientID(), client2.GetClientID(), client3.GetClientID())
	}
}

func TestGRPCTunnelClient_ClientIDFormat(t *testing.T) {
	// Reset global counter and process stable ID
	globalClientCounter = 0
	processStableClientID = ""

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
