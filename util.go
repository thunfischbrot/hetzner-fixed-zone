package hetzner

import (
	"fmt"
	"strings"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/libdns/libdns"
)

const (
	// minTTL is the minimum TTL in seconds accepted by the API.
	minTTL = 60
)

// unFQDN trims any trailing "." from fqdn. Hetzner's API does not use FQDNs.
func unFQDN(fqdn string) string {
	return strings.TrimSuffix(fqdn, ".")
}

func toRecord(name string, typ hcloud.ZoneRRSetType, ttl int, data string) (libdns.Record, error) {
	record, err := libdns.RR{
		Name: name,
		TTL:  time.Duration(ttl) * time.Second,
		Type: string(typ),
		Data: data,
	}.Parse()
	if err != nil {
		return nil, err
	}

	switch t := record.(type) {
	case libdns.TXT:
		// Remove enclosing quotation marks as documented in libdns.TXT.
		t.Text = strings.Trim(t.Text, "\"")

		return t, nil
	default:
		return record, nil
	}
}

func fromRecord(zone string, record libdns.Record) *hcloud.ZoneRRSet {
	switch t := record.(type) {
	case libdns.TXT:
		// Make sure TXT record data is enclosed in quotation marks.
		if !strings.HasPrefix(t.Text, "\"") || !strings.HasSuffix(t.Text, "\"") {
			t.Text = fmt.Sprintf("\"%s\"", t.Text)
		}

		record = t
	}

	rr := record.RR()

	// Ignore zero value. For non-zero values, make sure the TTL is at least minTTL.
	var ttl *int
	if sec := int(rr.TTL.Seconds()); sec != 0 {
		tmp := max(sec, minTTL)
		ttl = &tmp
	}

	return &hcloud.ZoneRRSet{
		Zone: &hcloud.Zone{
			Name: unFQDN(zone),
		},
		Name: strings.ToLower(rr.Name),
		Type: hcloud.ZoneRRSetType(strings.ToUpper(rr.Type)),
		TTL:  ttl,
		Records: []hcloud.ZoneRRSetRecord{{
			Value: rr.Data,
		}},
	}
}
