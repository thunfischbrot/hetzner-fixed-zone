package hetzner

import (
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// verify longest-suffix zone matching esp. for acme dns-01 subdomain delegation scenarios
func TestProvider_getZoneFromFQDN(t *testing.T) {
	testCases := []struct {
		name           string
		domain         string
		availableZones []string
		expectedZone   string
		expectError    bool
	}{
		{
			name:           "suffix match",
			domain:         "test.0testing.eu",
			availableZones: []string{"0testing.eu"},
			expectedZone:   "0testing.eu",
			expectError:    false,
		},
		{
			name:           "exact match",
			domain:         "0testing.eu",
			availableZones: []string{"0testing.eu"},
			expectedZone:   "0testing.eu",
			expectError:    false,
		},
		{
			name:           "longest suffix match",
			domain:         "sub.test.0testing.eu",
			availableZones: []string{"0testing.eu", "test.0testing.eu"},
			expectedZone:   "test.0testing.eu",
			expectError:    false,
		},
		{
			name:           "fail due to mismatch",
			domain:         "example.com",
			availableZones: []string{"0testing.eu", "test.0testing.eu"},
			expectedZone:   "",
			expectError:    true,
		},
		{
			name:           "ensure fail despite partial match", // "fake0testing.eu" contains "0testing.eu" but isn't acceptable suffix
			domain:         "fake0testing.eu",
			availableZones: []string{"0testing.eu"},
			expectedZone:   "",
			expectError:    true,
		},
		{
			name:           "multiple levels with longest match",
			domain:         "rly.not.my.president.com",
			availableZones: []string{"president.com", "my.president.com", "not.my.president.com"},
			expectedZone:   "not.my.president.com",
			expectError:    false,
		},
		{
			name:           "domain with trailing dot",
			domain:         "test.0testing.eu.",
			availableZones: []string{"0testing.eu"},
			expectedZone:   "0testing.eu",
			expectError:    false,
		},
		{
			name:           "zone with trailing dot",
			domain:         "test.0testing.eu",
			availableZones: []string{"0testing.eu."},
			expectedZone:   "0testing.eu",
			expectError:    false,
		},
		{
			name:           "both with trailing dots",
			domain:         "test.0testing.eu.",
			availableZones: []string{"0testing.eu."},
			expectedZone:   "0testing.eu",
			expectError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hcloudZones := make([]*hcloud.Zone, len(tc.availableZones))
			for i, name := range tc.availableZones {
				hcloudZones[i] = &hcloud.Zone{
					ID:   int64(i + 1),
					Name: name,
				}
			}

			// Test matching logic, w/o touching API
			result, err := getZoneFromList(tc.domain, hcloudZones)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("expected zone %q but got nil", tc.expectedZone)
				return
			}

			resultName := unFQDN(result.Name)
			if resultName != tc.expectedZone {
				t.Errorf("expected zone %q but got %q", tc.expectedZone, resultName)
			}
		})
	}
}
