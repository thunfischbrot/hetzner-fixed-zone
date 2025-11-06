package hetzner_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/libdns/hetzner/v2"
	"github.com/libdns/libdns"
)

var (
	envToken = ""
	envZone  = ""
	ttl      = 120 * time.Second
)

type testRecordsCleanup = func()

func setupTestRecords(t *testing.T, p *hetzner.Provider) ([]libdns.Record, testRecordsCleanup) {
	testRecords := []libdns.Record{
		libdns.TXT{
			Name: "test1",
			Text: "test1",
			TTL:  ttl,
		},
		libdns.TXT{
			Name: "test2",
			Text: "test2",
			TTL:  ttl,
		},
		libdns.TXT{
			Name: "test3",
			Text: "test3",
			TTL:  ttl,
		},
	}

	records, err := p.AppendRecords(t.Context(), envZone, testRecords)
	if err != nil {
		t.Fatal(err)
		return nil, func() {}
	}

	return records, func() {
		cleanupRecords(t, p, records)
	}
}

func cleanupRecords(t *testing.T, p *hetzner.Provider, r []libdns.Record) {
	_, err := p.DeleteRecords(t.Context(), envZone, r)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
}

func TestMain(m *testing.M) {
	envToken = os.Getenv("LIBDNS_HETZNER_TEST_TOKEN")
	envZone = os.Getenv("LIBDNS_HETZNER_TEST_ZONE")

	if len(envToken) == 0 || len(envZone) == 0 {
		fmt.Println(`Please note that this test runs against the public Hetzner DNS API, so you should
never run the test with a zone used in production.
To run this test, you have to specify 'LIBDNS_HETZNER_TEST_TOKEN' and 'LIBDNS_HETZNER_TEST_ZONE'.
Example: LIBDNS_HETZNER_TEST_TOKEN="123" LIBDNS_HETZNER_TEST_ZONE="my-domain.com" go test ./... -v`)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestProvider_AppendRecords(t *testing.T) {
	p := &hetzner.Provider{
		APIToken: envToken,
	}

	testCases := []struct {
		records  []libdns.Record
		expected []libdns.Record
	}{
		{
			// multiple records
			records: []libdns.Record{
				libdns.TXT{Name: "test_1", Text: "test_1", TTL: ttl},
				libdns.TXT{Name: "test_2", Text: "test_2", TTL: ttl},
				libdns.TXT{Name: "test_3", Text: "test_3", TTL: ttl},
			},
			expected: []libdns.Record{
				libdns.TXT{Name: "test_1", Text: "test_1", TTL: ttl},
				libdns.TXT{Name: "test_2", Text: "test_2", TTL: ttl},
				libdns.TXT{Name: "test_3", Text: "test_3", TTL: ttl},
			},
		},
		{
			// relative name
			records: []libdns.Record{
				libdns.TXT{Name: "123.test", Text: "123", TTL: ttl},
			},
			expected: []libdns.Record{
				libdns.TXT{Name: "123.test", Text: "123", TTL: ttl},
			},
		},
	}

	for _, c := range testCases {
		func() {
			result, err := p.AppendRecords(t.Context(), envZone+".", c.records)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanupRecords(t, p, result)

			if len(result) != len(c.records) {
				t.Fatalf("len(result) != len(c.records) => %d != %d", len(c.records), len(result))
			}

			for k, r := range result {
				resultRR := r.RR()
				expectedRR := c.expected[k].RR()

				if resultRR.Type != expectedRR.Type {
					t.Fatalf(".Type != c.expected[%d].Type => %s != %s", k, resultRR.Type, expectedRR.Type)
				}
				if resultRR.Name != expectedRR.Name {
					t.Fatalf("r.Name != c.expected[%d].Name => %s != %s", k, resultRR.Name, expectedRR.Name)
				}
				if resultRR.Data != expectedRR.Data {
					t.Fatalf("r.Data != c.expected[%d].Data => %s != %s", k, resultRR.Data, expectedRR.Data)
				}
				if resultRR.TTL != expectedRR.TTL {
					t.Fatalf("r.TTL != c.expected[%d].TTL => %s != %s", k, resultRR.TTL, expectedRR.TTL)
				}
			}
		}()
	}
}

func TestProvider_DeleteRecords(t *testing.T) {
	p := &hetzner.Provider{
		APIToken: envToken,
	}

	testRecords, cleanupFunc := setupTestRecords(t, p)
	defer cleanupFunc()

	records, err := p.DeleteRecords(t.Context(), envZone, testRecords)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) < len(testRecords) {
		t.Fatalf("len(records) < len(testRecords) => %d < %d", len(records), len(testRecords))
	}
}

func TestProvider_GetRecords(t *testing.T) {
	p := &hetzner.Provider{
		APIToken: envToken,
	}

	testRecords, cleanupFunc := setupTestRecords(t, p)
	defer cleanupFunc()

	records, err := p.GetRecords(t.Context(), envZone)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) < len(testRecords) {
		t.Fatalf("len(records) < len(testRecords) => %d < %d", len(records), len(testRecords))
	}
}

func TestProvider_SetRecords(t *testing.T) {
	p := &hetzner.Provider{
		APIToken: envToken,
	}

	existingRecords, _ := setupTestRecords(t, p)
	newTestRecords := []libdns.Record{
		libdns.TXT{
			Name: "new_test1",
			Text: "new_test1",
			TTL:  ttl,
		},
		libdns.TXT{
			Name: "new_test2",
			Text: "new_test2",
			TTL:  ttl,
		},
	}

	allRecords := append(existingRecords, newTestRecords...)
	allRecords[0] = libdns.TXT{
		Name: "test1",
		Text: "new_value",
		TTL:  ttl,
	}

	records, err := p.SetRecords(t.Context(), envZone, allRecords)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupRecords(t, p, records)

	if len(records) != len(allRecords) {
		t.Fatalf("len(records) != len(allRecords) => %d != %d", len(records), len(allRecords))
	}

	if rr := records[0].RR(); rr.Data != "new_value" {
		t.Fatalf(`records[0].Value != "new_value" => %s != "new_value"`, rr.Data)
	}
}

func TestProvider_ListZones(t *testing.T) {
	p := &hetzner.Provider{
		APIToken: envToken,
	}

	_, err := p.ListZones(t.Context())
	if err != nil {
		t.Fatal(err)
	}
}
