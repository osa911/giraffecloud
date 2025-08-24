package tunnel

import (
	"errors"
	"testing"
)

func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "timeout error",
			err:      errors.New("timeout"),
			expected: true,
		},
		{
			name:     "deadline exceeded",
			err:      errors.New("context deadline exceeded"),
			expected: true,
		},
		{
			name:     "tunnel inactive",
			err:      errors.New("tunnel inactive"),
			expected: true,
		},
		{
			name:     "connection timed out",
			err:      errors.New("connection timed out"),
			expected: true,
		},
		{
			name:     "i/o timeout",
			err:      errors.New("i/o timeout"),
			expected: true,
		},
		{
			name:     "regular error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimeoutError(tt.err)
			if result != tt.expected {
				t.Errorf("isTimeoutError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestGRPCTunnelClientMetrics(t *testing.T) {
	client := &GRPCTunnelClient{
		clientID: "test-client",
		domain:   "test.com",
	}

	// Test initial metrics
	metrics := client.GetMetrics()
	if metrics["total_errors"] != int64(0) {
		t.Errorf("Expected initial total_errors to be 0, got %v", metrics["total_errors"])
	}
	if metrics["timeout_errors"] != int64(0) {
		t.Errorf("Expected initial timeout_errors to be 0, got %v", metrics["timeout_errors"])
	}
	if metrics["reconnect_count"] != int64(0) {
		t.Errorf("Expected initial reconnect_count to be 0, got %v", metrics["reconnect_count"])
	}
	if metrics["timeout_reconnects"] != int64(0) {
		t.Errorf("Expected initial timeout_reconnects to be 0, got %v", metrics["timeout_reconnects"])
	}
}
