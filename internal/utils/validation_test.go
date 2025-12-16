package utils

import "testing"

func TestIsValidDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"sub-domain.example.com", true},
		{"123.com", true},
		{"example.co.uk", true},
		{"localhost", false},
		{"invalid", false},
		{"example", false},
		{"ex_ample.com", false}, // Underscore not allowed
		{"example.c", false},    // TLD too short
		{"192.168.1.1", false},  // IP address
		{"-example.com", false}, // Starts with hyphen
		{"example-.com", false}, // Ends with hyphen
		{"", false},             // Empty
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			if got := IsValidDomain(tt.domain); got != tt.want {
				t.Errorf("IsValidDomain(%q) = %v, want %v", tt.domain, got, tt.want)
			}
		})
	}
}
