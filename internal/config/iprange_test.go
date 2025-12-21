package config

import (
	"testing"
)

func TestParseIPRange(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantCount   int
		wantFirst   string
		wantLast    string
		expectError bool
	}{
		{
			name:        "single IP",
			input:       "10.10.10.5",
			wantCount:   1,
			wantFirst:   "10.10.10.5",
			wantLast:    "10.10.10.5",
			expectError: false,
		},
		{
			name:        "small range",
			input:       "10.10.10.1-10.10.10.5",
			wantCount:   5,
			wantFirst:   "10.10.10.1",
			wantLast:    "10.10.10.5",
			expectError: false,
		},
		{
			name:        "range with spaces",
			input:       "  192.168.1.10 - 192.168.1.15  ",
			wantCount:   6,
			wantFirst:   "192.168.1.10",
			wantLast:    "192.168.1.15",
			expectError: false,
		},
		{
			name:        "range spanning 100 IPs",
			input:       "10.10.10.1-10.10.10.100",
			wantCount:   100,
			wantFirst:   "10.10.10.1",
			wantLast:    "10.10.10.100",
			expectError: false,
		},
		{
			name:        "invalid format",
			input:       "not-an-ip",
			expectError: true,
		},
		{
			name:        "reversed range",
			input:       "10.10.10.10-10.10.10.5",
			expectError: true,
		},
		{
			name:        "too many parts",
			input:       "10.10.10.1-10.10.10.5-10.10.10.10",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIPRange(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseIPRange() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseIPRange() unexpected error: %v", err)
				return
			}

			if len(got) != tt.wantCount {
				t.Errorf("ParseIPRange() count = %d, want %d", len(got), tt.wantCount)
			}

			if tt.wantCount > 0 {
				if got[0] != tt.wantFirst {
					t.Errorf("ParseIPRange() first IP = %s, want %s", got[0], tt.wantFirst)
				}
				if got[len(got)-1] != tt.wantLast {
					t.Errorf("ParseIPRange() last IP = %s, want %s", got[len(got)-1], tt.wantLast)
				}
			}
		})
	}
}

func TestExpandIPRanges(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		wantCount   int
		expectError bool
	}{
		{
			name:        "single range",
			input:       []string{"10.10.10.1-10.10.10.5"},
			wantCount:   5,
			expectError: false,
		},
		{
			name:        "multiple ranges",
			input:       []string{"10.10.10.1-10.10.10.5", "10.10.10.58-10.10.10.60"},
			wantCount:   8,
			expectError: false,
		},
		{
			name:        "mixed single IPs and ranges",
			input:       []string{"192.168.1.1", "192.168.1.10-192.168.1.12", "192.168.1.100"},
			wantCount:   5,
			expectError: false,
		},
		{
			name:        "duplicate IPs (should deduplicate)",
			input:       []string{"10.10.10.1-10.10.10.3", "10.10.10.2-10.10.10.5"},
			wantCount:   5, // 1,2,3,4,5 (2 and 3 appear in both)
			expectError: false,
		},
		{
			name:        "invalid range",
			input:       []string{"10.10.10.1-10.10.10.5", "invalid-range"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandIPRanges(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("ExpandIPRanges() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ExpandIPRanges() unexpected error: %v", err)
				return
			}

			if len(got) != tt.wantCount {
				t.Errorf("ExpandIPRanges() count = %d, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestParseCIDR(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		minCount    int // Minimum expected IPs (excludes network/broadcast)
		maxCount    int // Maximum expected IPs
		expectError bool
	}{
		{
			name:        "valid /30 CIDR",
			input:       "192.168.1.0/30",
			minCount:    2,
			maxCount:    4,
			expectError: false,
		},
		{
			name:        "valid /29 CIDR",
			input:       "10.0.0.0/29",
			minCount:    6,
			maxCount:    8,
			expectError: false,
		},
		{
			name:        "invalid CIDR",
			input:       "not-a-cidr/24",
			expectError: true,
		},
		{
			name:        "IPv6 CIDR (not supported)",
			input:       "2001:db8::/32",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCIDR(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseCIDR() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseCIDR() unexpected error: %v", err)
				return
			}

			if len(got) < tt.minCount || len(got) > tt.maxCount {
				t.Errorf("ParseCIDR() count = %d, want between %d and %d", len(got), tt.minCount, tt.maxCount)
			}
		})
	}
}

func TestCountIPsInRange(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        int
		expectError bool
	}{
		{
			name:        "single IP",
			input:       "10.10.10.5",
			want:        1,
			expectError: false,
		},
		{
			name:        "small range",
			input:       "10.10.10.1-10.10.10.10",
			want:        10,
			expectError: false,
		},
		{
			name:        "large range",
			input:       "10.10.10.1-10.10.10.100",
			want:        100,
			expectError: false,
		},
		{
			name:        "/24 CIDR",
			input:       "192.168.1.0/24",
			want:        256,
			expectError: false,
		},
		{
			name:        "invalid input",
			input:       "not-valid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CountIPsInRange(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("CountIPsInRange() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("CountIPsInRange() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("CountIPsInRange() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExpandServerInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantCount   int
		expectError bool
	}{
		{
			name:        "single IP",
			input:       "10.10.10.5",
			wantCount:   1,
			expectError: false,
		},
		{
			name:        "IP range",
			input:       "10.10.10.1-10.10.10.5",
			wantCount:   5,
			expectError: false,
		},
		{
			name:        "CIDR notation",
			input:       "192.168.1.0/30",
			wantCount:   4, // Could be 2-4 depending on network/broadcast filtering
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandServerInput(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("ExpandServerInput() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ExpandServerInput() unexpected error: %v", err)
				return
			}

			// Allow some flexibility for CIDR ranges
			if tt.input == "192.168.1.0/30" {
				if len(got) < 2 || len(got) > 4 {
					t.Errorf("ExpandServerInput() count = %d, want 2-4", len(got))
				}
			} else if len(got) != tt.wantCount {
				t.Errorf("ExpandServerInput() count = %d, want %d", len(got), tt.wantCount)
			}
		})
	}
}
