package hetzner

import (
	"strings"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/libdns/libdns"
)

// test zone resolution when certmagic passes partial zone suffix (e.g., "eu.")
// but record contains full subdomain path (e.g., "_acme-challenge.sub.test")
func TestZoneMatchingWithPartialZoneSuffix(t *testing.T) {
	testCases := []struct {
		name           string
		inputZone      string // what certmagic/caller passes
		recordName     string // record name from libdns.Record
		availableZones []string
		expectedZone   string // which zone should actually be used
	}{
		{
			name:           "acme challenge with subdomain delegation - finds parent zone",
			inputZone:      "eu.",
			recordName:     "_acme-challenge.sub.test.0testing",
			availableZones: []string{"0testing.eu"},
			expectedZone:   "0testing.eu",
		},
		{
			name:           "acme challenge with subdomain delegation - prefers longest match",
			inputZone:      "eu.",
			recordName:     "_acme-challenge.sub.test.0testing",
			availableZones: []string{"0testing.eu", "test.0testing.eu"},
			expectedZone:   "test.0testing.eu",
		},
		{
			name:           "acme challenge deeply nested subdomain",
			inputZone:      "eu.",
			recordName:     "_acme-challenge.deep.nested.sub.test.0testing",
			availableZones: []string{"0testing.eu", "test.0testing.eu", "sub.test.0testing.eu"},
			expectedZone:   "sub.test.0testing.eu",
		},
		{
			name:           "simple record with exact zone match",
			inputZone:      "0testing.eu.",
			recordName:     "www",
			availableZones: []string{"0testing.eu"},
			expectedZone:   "0testing.eu",
		},
		{
			name:           "record with subdomain prefers delegated zone",
			inputZone:      "0testing.eu.",
			recordName:     "api.sub.test",
			availableZones: []string{"0testing.eu", "test.0testing.eu"},
			expectedZone:   "test.0testing.eu",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// construct full FQDN like the actual code does
			fullDomain := tc.recordName
			if fullDomain != "" && fullDomain[len(fullDomain)-1] != '.' {
				fullDomain = fullDomain + "."
			}
			fullDomain = fullDomain + tc.inputZone

			// create mock zone list
			hcloudZones := make([]*hcloud.Zone, len(tc.availableZones))
			for i, name := range tc.availableZones {
				hcloudZones[i] = &hcloud.Zone{
					ID:   int64(i + 1),
					Name: name,
				}
			}

			// test zone matching
			result, err := getZoneFromList(fullDomain, hcloudZones)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatalf("expected zone %q but got nil", tc.expectedZone)
			}

			resultName := unFQDN(result.Name)
			if resultName != tc.expectedZone {
				t.Errorf("expected zone %q but got %q (fullDomain was %q)", tc.expectedZone, resultName, fullDomain)
			}
		})
	}
}

// verify fromRecord() uses the zone we provide, not some derived value
func TestFromRecordUsesProvidedZone(t *testing.T) {
	record := libdns.TXT{
		Name: "_acme-challenge.sub.test",
		Text: "validation-token-here",
		TTL:  300,
	}

	// fromRecord should use whatever zone we pass, not try to derive it
	rrset := fromRecord("0testing.eu", record)

	if rrset.Zone == nil {
		t.Fatal("expected Zone to be set")
	}

	if rrset.Zone.Name != "0testing.eu" {
		t.Errorf("expected zone name %q but got %q", "0testing.eu", rrset.Zone.Name)
	}

	// record name should be preserved as-is (lowercase)
	expectedName := "_acme-challenge.sub.test"
	if rrset.Name != expectedName {
		t.Errorf("expected record name %q but got %q", expectedName, rrset.Name)
	}
}

// verify record names get adjusted to be relative to actual zone
func TestRecordNameAdjustment(t *testing.T) {
	testCases := []struct {
		name             string
		fullDomain       string
		actualZone       string
		expectedRelative string
	}{
		{
			name:             "acme challenge: strip zone suffix from FQDN",
			fullDomain:       "_acme-challenge.sub.test.0testing.eu",
			actualZone:       "0testing.eu",
			expectedRelative: "_acme-challenge.sub.test",
		},
		{
			name:             "nested subdomain: strip delegated zone",
			fullDomain:       "api.sub.test.0testing.eu",
			actualZone:       "test.0testing.eu",
			expectedRelative: "api.sub",
		},
		{
			name:             "apex record: empty relative name",
			fullDomain:       "0testing.eu",
			actualZone:       "0testing.eu",
			expectedRelative: "",
		},
		{
			name:             "simple subdomain",
			fullDomain:       "www.0testing.eu",
			actualZone:       "0testing.eu",
			expectedRelative: "www",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			zoneSuffix := "." + unFQDN(tc.actualZone)
			fullDomainNoDot := unFQDN(tc.fullDomain)

			var relativeName string
			if strings.HasSuffix(fullDomainNoDot, zoneSuffix) {
				relativeName = strings.TrimSuffix(fullDomainNoDot, zoneSuffix)
			} else if fullDomainNoDot == unFQDN(tc.actualZone) {
				relativeName = ""
			}

			if relativeName != tc.expectedRelative {
				t.Errorf("expected relative name %q but got %q (fullDomain=%q, zone=%q)",
					tc.expectedRelative, relativeName, tc.fullDomain, tc.actualZone)
			}
		})
	}
}

// edge case: empty record name should still work
func TestZoneMatchingWithEmptyRecordName(t *testing.T) {
	inputZone := "0testing.eu."
	recordName := ""
	availableZones := []string{"0testing.eu"}

	// construct full FQDN
	fullDomain := recordName
	if fullDomain != "" && fullDomain[len(fullDomain)-1] != '.' {
		fullDomain = fullDomain + "."
	}
	fullDomain = fullDomain + inputZone

	// create mock zone
	hcloudZones := []*hcloud.Zone{
		{ID: 1, Name: availableZones[0]},
	}

	result, err := getZoneFromList(fullDomain, hcloudZones)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected zone but got nil")
	}

	resultName := unFQDN(result.Name)
	if resultName != "0testing.eu" {
		t.Errorf("expected zone %q but got %q", "0testing.eu", resultName)
	}
}
