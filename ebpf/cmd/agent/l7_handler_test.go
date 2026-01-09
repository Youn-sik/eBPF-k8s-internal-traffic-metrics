package main

import (
	"testing"
)

func TestHealthCheckFilter_NewHealthCheckFilter(t *testing.T) {
	tests := []struct {
		name           string
		enabled        bool
		customPatterns string
		wantEnabled    bool
		wantMinCount   int
	}{
		{
			name:           "enabled with defaults",
			enabled:        true,
			customPatterns: "",
			wantEnabled:    true,
			wantMinCount:   9, // default patterns count
		},
		{
			name:           "disabled",
			enabled:        false,
			customPatterns: "",
			wantEnabled:    false,
			wantMinCount:   9,
		},
		{
			name:           "with custom patterns",
			enabled:        true,
			customPatterns: "/custom-health,/app-status",
			wantEnabled:    true,
			wantMinCount:   11, // 9 defaults + 2 custom
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewHealthCheckFilter(tt.enabled, tt.customPatterns)

			if filter.enabled != tt.wantEnabled {
				t.Errorf("enabled = %v, want %v", filter.enabled, tt.wantEnabled)
			}

			if len(filter.patterns) < tt.wantMinCount {
				t.Errorf("got %d patterns, want at least %d", len(filter.patterns), tt.wantMinCount)
			}
		})
	}
}

func TestHealthCheckFilter_IsHealthCheck(t *testing.T) {
	filter := NewHealthCheckFilter(true, "/custom-health")

	tests := []struct {
		path string
		want bool
	}{
		// Default patterns
		{"/healthz", true},
		{"/readyz", true},
		{"/livez", true},
		{"/health", true},
		{"/ready", true},
		{"/live", true},
		{"/ping", true},
		{"/status", true},
		{"/_health", true},

		// With subpaths
		{"/healthz/live", true},
		{"/health/check", true},
		{"/status/ready", true},

		// Custom pattern
		{"/custom-health", true},
		{"/custom-health/deep", true},

		// Case insensitive
		{"/HEALTHZ", true},
		{"/Health", true},
		{"/PING", true},

		// Not health checks
		{"/api/users", false},
		{"/api/health-data", false}, // health is not prefix
		{"/v1/status-report", false},
		{"/metrics", false},
		{"/", false},
		{"/api", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := filter.IsHealthCheck(tt.path)
			if got != tt.want {
				t.Errorf("IsHealthCheck(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestHealthCheckFilter_Disabled(t *testing.T) {
	filter := NewHealthCheckFilter(false, "")

	// When disabled, nothing should be considered a health check
	paths := []string{"/healthz", "/readyz", "/health", "/ping"}

	for _, path := range paths {
		if filter.IsHealthCheck(path) {
			t.Errorf("disabled filter should not match %q", path)
		}
	}
}

func TestHealthCheckFilter_Patterns(t *testing.T) {
	filter := NewHealthCheckFilter(true, "/custom1,/custom2")
	patterns := filter.Patterns()

	// Should include defaults + custom
	if len(patterns) < 11 {
		t.Errorf("got %d patterns, want at least 11", len(patterns))
	}

	// Check custom patterns are included
	customFound := 0
	for _, p := range patterns {
		if p == "/custom1" || p == "/custom2" {
			customFound++
		}
	}

	if customFound != 2 {
		t.Errorf("expected 2 custom patterns, found %d", customFound)
	}
}

func TestBytesToString(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{
			name:  "null terminated",
			input: []byte{'h', 'e', 'l', 'l', 'o', 0, 'x', 'x'},
			want:  "hello",
		},
		{
			name:  "no null",
			input: []byte{'h', 'e', 'l', 'l', 'o'},
			want:  "hello",
		},
		{
			name:  "empty",
			input: []byte{0},
			want:  "",
		},
		{
			name:  "all null",
			input: []byte{0, 0, 0, 0},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bytesToString(tt.input)
			if got != tt.want {
				t.Errorf("bytesToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUint32ToIP(t *testing.T) {
	tests := []struct {
		name  string
		input uint32
		want  string
	}{
		{
			name:  "localhost",
			input: 0x7f000001, // 127.0.0.1 in big endian
			want:  "127.0.0.1",
		},
		{
			name:  "10.0.0.1",
			input: 0x0a000001,
			want:  "10.0.0.1",
		},
		{
			name:  "192.168.1.1",
			input: 0xc0a80101,
			want:  "192.168.1.1",
		},
		{
			name:  "zero",
			input: 0,
			want:  "0.0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uint32ToIP(tt.input)
			if got.String() != tt.want {
				t.Errorf("uint32ToIP(0x%x) = %s, want %s", tt.input, got.String(), tt.want)
			}
		})
	}
}
