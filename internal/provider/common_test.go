package provider

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Unit tests for common.go utility functions
//
// All three functions are pure (no I/O, no state) so they are tested
// directly with table-driven tests — no mock server needed.
// ---------------------------------------------------------------------------

func TestUnitPlatformVersionAtLeast(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		minimum  string
		expected bool
	}{
		{
			name:     "equal version",
			version:  "1.7.0-12345",
			minimum:  "v1.7",
			expected: true,
		},
		{
			name:     "above minimum",
			version:  "1.8.3-23453",
			minimum:  "v1.7",
			expected: true,
		},
		{
			name:     "below minimum",
			version:  "1.5.0-10234",
			minimum:  "v1.7",
			expected: false,
		},
		{
			name:     "version with v prefix",
			version:  "v1.8.0-5000",
			minimum:  "v1.7",
			expected: true,
		},
		{
			name:     "empty version returns false",
			version:  "",
			minimum:  "v1.7",
			expected: false,
		},
		{
			name:     "exact major minor match",
			version:  "1.7.1-99999",
			minimum:  "v1.7",
			expected: true,
		},
		{
			name:     "major version higher",
			version:  "2.0.0-1",
			minimum:  "v1.7",
			expected: true,
		},
		{
			name:     "major version lower",
			version:  "0.9.0-1",
			minimum:  "v1.7",
			expected: false,
		},
		{
			name:     "higher minimum",
			version:  "1.8.0-100",
			minimum:  "v1.9",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := platformVersionAtLeast(tc.version, tc.minimum)
			if result != tc.expected {
				t.Errorf("platformVersionAtLeast(%q, %q) = %v, want %v",
					tc.version, tc.minimum, result, tc.expected)
			}
		})
	}
}

func TestUnitExtractSubnet(t *testing.T) {
	tests := []struct {
		name         string
		cidr         string
		expectedBits int
		expectedIP   string
		expectErr    bool
	}{
		{
			name:         "standard /24",
			cidr:         "192.168.1.100/24",
			expectedBits: 24,
			expectedIP:   "192.168.1.100",
		},
		{
			name:         "/32 host route",
			cidr:         "10.0.0.1/32",
			expectedBits: 32,
			expectedIP:   "10.0.0.1",
		},
		{
			name:         "/8 class A",
			cidr:         "10.255.0.1/8",
			expectedBits: 8,
			expectedIP:   "10.255.0.1",
		},
		{
			name:         "IPv6 /64",
			cidr:         "2001:db8::1/64",
			expectedBits: 64,
			expectedIP:   "2001:db8::1",
		},
		{
			name:      "invalid CIDR",
			cidr:      "not-a-cidr",
			expectErr: true,
		},
		{
			name:      "missing prefix length",
			cidr:      "192.168.1.1",
			expectErr: true,
		},
		{
			name:      "empty string",
			cidr:      "",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bits, ip, err := extractSubnet(tc.cidr)
			if tc.expectErr {
				if err == nil {
					t.Errorf("extractSubnet(%q) expected error, got bits=%d ip=%q",
						tc.cidr, bits, ip)
				}
				return
			}
			if err != nil {
				t.Fatalf("extractSubnet(%q) unexpected error: %v", tc.cidr, err)
			}
			if bits != tc.expectedBits {
				t.Errorf("extractSubnet(%q) bits = %d, want %d",
					tc.cidr, bits, tc.expectedBits)
			}
			if ip != tc.expectedIP {
				t.Errorf("extractSubnet(%q) ip = %q, want %q",
					tc.cidr, ip, tc.expectedIP)
			}
		})
	}
}

func TestUnitGetIntSliceDifference(t *testing.T) {
	tests := []struct {
		name     string
		slice1   []int64
		slice2   []int64
		expected []int64
	}{
		{
			name:     "some elements removed",
			slice1:   []int64{1, 2, 3, 4, 5},
			slice2:   []int64{2, 4},
			expected: []int64{1, 3, 5},
		},
		{
			name:     "no overlap",
			slice1:   []int64{1, 2, 3},
			slice2:   []int64{4, 5, 6},
			expected: []int64{1, 2, 3},
		},
		{
			name:     "identical slices",
			slice1:   []int64{1, 2, 3},
			slice2:   []int64{1, 2, 3},
			expected: nil,
		},
		{
			name:     "empty first slice",
			slice1:   []int64{},
			slice2:   []int64{1, 2, 3},
			expected: nil,
		},
		{
			name:     "empty second slice",
			slice1:   []int64{1, 2, 3},
			slice2:   []int64{},
			expected: []int64{1, 2, 3},
		},
		{
			name:     "both empty",
			slice1:   []int64{},
			slice2:   []int64{},
			expected: nil,
		},
		{
			name:     "nil first slice",
			slice1:   nil,
			slice2:   []int64{1, 2},
			expected: nil,
		},
		{
			name:     "nil second slice",
			slice1:   []int64{10, 20},
			slice2:   nil,
			expected: []int64{10, 20},
		},
		{
			name:     "single element difference",
			slice1:   []int64{42},
			slice2:   []int64{99},
			expected: []int64{42},
		},
		{
			name:     "single element match",
			slice1:   []int64{42},
			slice2:   []int64{42},
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := getIntSliceDifference(tc.slice1, tc.slice2)
			if tc.expected == nil {
				if result != nil {
					t.Errorf("getIntSliceDifference(%v, %v) = %v, want nil",
						tc.slice1, tc.slice2, result)
				}
				return
			}
			if len(result) != len(tc.expected) {
				t.Fatalf("getIntSliceDifference(%v, %v) = %v (len %d), want %v (len %d)",
					tc.slice1, tc.slice2, result, len(result), tc.expected, len(tc.expected))
			}
			for i, v := range result {
				if v != tc.expected[i] {
					t.Errorf("getIntSliceDifference(%v, %v)[%d] = %d, want %d",
						tc.slice1, tc.slice2, i, v, tc.expected[i])
				}
			}
		})
	}
}
