// Package hetzner implements the [libdns](https://github.com/libdns/libdns) interfaces for the Hetzner Cloud DNS API.
package hetzner

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/libdns/libdns"
	"golang.org/x/net/idna"
)

const (
	applicationName    = "github.com/libdns/hetzner"
	applicationVersion = "v2"

	envVarName = "LIBDNS_HETZNER_TOKEN"
)

// Provider implements the libdns interfaces for the Hetzner DNS API.
type Provider struct {
	// APIToken is the Hetzner Cloud API token to use with HTTP requests. If not set, it defaults to the
	// LIBDNS_HETZNER_TOKEN environment variable.
	//
	// See https://docs.hetzner.cloud/reference/cloud#getting-started to learn how to generate a new token.
	APIToken string `json:"api_token,omitempty"`

	client *hcloud.Client
	once   sync.Once
}

// New returns a new libdns provider for Hetzner.
//
// If token is empty, the provider will default to the LIBDNS_HETZNER_TOKEN environment variable.
func New(token string) *Provider {
	return &Provider{
		APIToken: token,
	}
}

// getClient returns a hcloud.Client configured with the Provider's APIToken. The client is bootstrapped in the first
// call to this function and then re-used for subsequent calls.
func (p *Provider) getClient() *hcloud.Client {
	p.once.Do(func() {
		token := p.APIToken

		if token == "" {
			token = os.Getenv(envVarName)
		}

		if token == "" {
			panic("hetzner: API token missing")
		}

		p.client = hcloud.NewClient(
			hcloud.WithToken(token),
			hcloud.WithApplication(applicationName, applicationVersion),
		)
	})

	return p.client
}

// getZoneFromFQDN resolve domain from zone using lookup + longest-suffix match.
// handles subdomain delegation: "*.sub.0testing.eu" finds "0testing.eu" when that's the only zone,
// but prefers "sub.0testing.eu" if both exist (longest wins).
func (p *Provider) getZoneFromFQDN(ctx context.Context, domain string) (*hcloud.Zone, error) {
	zones, err := p.getClient().Zone.All(ctx)
	if err != nil {
		return nil, err
	}

	return getZoneFromList(domain, zones)
}

// getZoneFromList does the actual suffix matching math. strip trailing dots,
// walk zone list,  return longest match.
func getZoneFromList(domain string, zones []*hcloud.Zone) (*hcloud.Zone, error) {
	domain = unFQDN(domain)

	var bestZone *hcloud.Zone
	var longestMatch int

	for _, zone := range zones {
		log.Printf("DEBUG: checking zone=%s", zone.Name)
		zoneName := unFQDN(zone.Name)
		if domain == zoneName || strings.HasSuffix(domain, "."+zoneName) {
			log.Printf("DEBUG: zone %s matches!", zone.Name)
			if len(zoneName) > longestMatch {
				longestMatch = len(zoneName)
				bestZone = zone
			}
		}
	}

	if bestZone == nil {
		return nil, fmt.Errorf("this should not happen: no matching zone found for domain '%s'", domain)
	}
	log.Printf("DEBUG: selected zone=%s", bestZone.Name)
	return bestZone, nil
}

// GetRecords returns all the records in the DNS zone. This function fulfills the libdns.RecordGetter interface.
//
// This implementation includes DNSSEC-related records.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	zone, err := idna.Lookup.ToASCII(zone)
	if err != nil {
		return nil, err
	}

	hcloudZone, err := p.getZoneFromFQDN(ctx, zone)
	if err != nil {
		return nil, err
	}

	sets, err := p.getClient().Zone.AllRRSets(ctx, hcloudZone)
	if err != nil {
		return nil, err
	}

	var records []libdns.Record

	for _, set := range sets {
		// An RRset may not have a TTL set explicitly, in which case the zone's default TTL is used.
		ttl := hcloudZone.TTL
		if set.TTL != nil {
			ttl = *set.TTL
		}

		for _, record := range set.Records {
			rr, err := toRecord(set.Name, set.Type, ttl, record.Value)
			if err != nil {
				return nil, err
			}

			records = append(records, rr)
		}
	}

	return records, nil
}

// AppendRecords creates the inputted records in the given zone and returns the populated records that were created.
// This function fulfills the libdns.RecordAppender interface.
//
// Please note that appending records may fail if the TTL doesn't match an existing RRset. For example,
//
//	;; Original zone
//	example.com. 3600 IN TXT "hello world"
//
//	;; Input
//	example.com. 60   IN TXT "hello world as well"
//
// will fail, as the given TTL of 60s conflicts with TTL 3600s of the existing RRset identified by (example.com., TXT).
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) (
	[]libdns.Record, error,
) {
	zone, err := idna.Lookup.ToASCII(zone)
	if err != nil {
		return nil, err
	}

	hcloudZone, err := p.getZoneFromFQDN(ctx, zone)
	if err != nil {
		return nil, err
	}

	var actions []*hcloud.Action

	for _, record := range records {
		// records are not always concrete types (libdns.Address, libdns.TXT, ...) but instead of type libdns.RR.
		// Parse them again so the type-switch in fromRecord() works correctly.
		record, err := record.RR().Parse()
		if err != nil {
			return nil, err
		}

		set := fromRecord(hcloudZone.Name, record)

		action, _, err := p.getClient().Zone.AddRRSetRecords(ctx, set, hcloud.ZoneRRSetAddRecordsOpts{
			Records: set.Records,
			TTL:     set.TTL,
		})
		if err != nil {
			return nil, err
		}

		actions = append(actions, action)
	}

	return records, p.getClient().Action.WaitFor(ctx, actions...)
}

// SetRecords updates the zone so that the records described in the input are reflected in the output. It returns the
// records which were set. Errors may result in partial changes to the zone. This function fulfills the
// libdns.RecordSetter interface.
//
// This implementation supports setting DNSSEC-related records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) (
	[]libdns.Record, error,
) {
	zone, err := idna.Lookup.ToASCII(zone)
	if err != nil {
		return nil, err
	}

	hcloudZone, err := p.getZoneFromFQDN(ctx, zone)
	if err != nil {
		return nil, err
	}

	type rrset struct {
		Name string
		Type string
	}

	sets := make(map[rrset]*hcloud.ZoneRRSet)

	// Group records by RRset.
	for _, record := range records {
		// records are not always concrete types (libdns.Address, libdns.TXT, ...) but instead of type libdns.RR.
		// Parse them again so the type-switch in fromRecord() works correctly.
		record, err := record.RR().Parse()
		if err != nil {
			return nil, err
		}

		set := fromRecord(hcloudZone.Name, record)

		key := rrset{
			Name: set.Name,
			Type: string(set.Type),
		}

		if _, ok := sets[key]; !ok {
			sets[key] = set
		} else {
			sets[key].Records = append(sets[key].Records, set.Records...)

			// fromRecord always sets a TTL, no need for nil check.
			if *set.TTL < *sets[key].TTL {
				sets[key].TTL = set.TTL
			}
		}
	}

	var actions []*hcloud.Action

	// Delete each possibly existing RRset, then re-create if with the appropriate records. Re-creating the RRset allows
	// setting the TTL in the same API call; otherwise a SetRRSetRecords followed by ChangeRRSetTTL would be necessary.
	for _, set := range sets {
		deleteRes, _, err := p.getClient().Zone.DeleteRRSet(ctx, set)
		if err != nil {
			// Ignore HTTP 404 errors.
			if !hcloud.IsError(err, hcloud.ErrorCodeNotFound) {
				return nil, err
			}
		}

		createRes, _, err := p.getClient().Zone.CreateRRSet(ctx, hcloudZone, hcloud.ZoneRRSetCreateOpts{
			Name:    set.Name,
			Type:    set.Type,
			TTL:     set.TTL,
			Records: set.Records,
		})
		if err != nil {
			return nil, err
		}

		actions = append(actions, deleteRes.Action, createRes.Action)
	}

	return records, p.getClient().Action.WaitFor(ctx, actions...)
}

// DeleteRecords deletes the given records from the zone if they exist in the zone and exactly match the input. If the
// input records do not exist in the zone, they are silently ignored. DeleteRecords returns only the records that were
// deleted, and does not return any records that were provided in the input but did not exist in the zone. This function
// fulfills the libdns.RecordDeleter interface.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) (
	[]libdns.Record, error,
) {
	zone, err := idna.Lookup.ToASCII(zone)
	if err != nil {
		return nil, err
	}

	hcloudZone, err := p.getZoneFromFQDN(ctx, zone)
	if err != nil {
		return nil, err
	}

	var (
		deleted []libdns.Record
		actions []*hcloud.Action
	)

	for _, record := range records {
		// records are not always concrete types (libdns.Address, libdns.TXT, ...) but instead of type libdns.RR.
		// Parse them again so the type-switch in fromRecord() works correctly.
		record, err := record.RR().Parse()
		if err != nil {
			return nil, err
		}

		set := fromRecord(hcloudZone.Name, record)

		action, _, err := p.getClient().Zone.RemoveRRSetRecords(ctx, set, hcloud.ZoneRRSetRemoveRecordsOpts{
			Records: set.Records,
		})
		if err != nil {
			// Ignore HTTP 404 errors.
			if hcloud.IsError(err, hcloud.ErrorCodeNotFound) {
				continue
			}

			return nil, err
		}

		deleted = append(deleted, record)
		actions = append(actions, action)
	}

	return deleted, p.getClient().Action.WaitFor(ctx, actions...)
}

// ListZones returns the list of available DNS zones for use by other functions. This function fulfills the
// libdns.ZoneLister interface.
func (p *Provider) ListZones(ctx context.Context) ([]libdns.Zone, error) {
	hcloudZones, err := p.getClient().Zone.All(ctx)
	if err != nil {
		return nil, err
	}

	zones := make([]libdns.Zone, len(hcloudZones))

	for i, zone := range hcloudZones {
		zones[i] = libdns.Zone{Name: zone.Name}
	}

	return zones, nil
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
	_ libdns.ZoneLister     = (*Provider)(nil)
)
