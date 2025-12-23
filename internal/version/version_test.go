package version

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"v1.0.542", "v1.0.533", 1},
		{"v1.0.533", "v1.0.542", -1},
		{"1.0.542", "1.0.542", 0},
		{"v1.0.542", "1.0.542", 0},
		{"v1.0.10", "v1.0.2", 1},
		{"v1.0.2", "v1.0.10", -1},
		{"dev", "v1.0.0", -1},
		{"v1.0.0", "dev", 1},
		{"v1.0.0-test.1", "v1.0.0", -1},
		{"v1.0.0", "v1.0.0-test.1", 1},
	}

	for _, tt := range tests {
		result := CompareVersions(tt.v1, tt.v2)
		if result != tt.expected {
			t.Errorf("CompareVersions(%s, %s) = %d; want %d", tt.v1, tt.v2, result, tt.expected)
		}
	}
}

func TestIsUpdateRequired(t *testing.T) {
	tests := []struct {
		client   string
		minimum  string
		expected bool
	}{
		{"v1.0.542", "v1.0.533", false},
		{"v1.0.533", "v1.0.542", true},
		{"v1.0.533", "v1.0.533", false},
		{"v1.0.542", "v1.0.542", false},
	}

	for _, tt := range tests {
		result := IsUpdateRequired(tt.client, tt.minimum)
		if result != tt.expected {
			t.Errorf("IsUpdateRequired(%s, %s) = %v; want %v", tt.client, tt.minimum, result, tt.expected)
		}
	}
}
