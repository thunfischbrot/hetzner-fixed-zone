package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/libdns/hetzner/v2"
)

func main() {
	var (
		token = os.Getenv("LIBDNS_HETZNER_TOKEN")
		zone  = os.Getenv("LIBDNS_HETZNER_ZONE")
	)

	if token == "" || zone == "" {
		fmt.Println("LIBDNS_HETZNER_TOKEN and/or LIBDNS_HETZNER_ZONE not set")
		return
	}

	p := hetzner.Provider{
		APIToken: token,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	records, err := p.GetRecords(ctx, zone)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return
	}

	for _, record := range records {
		fmt.Println(record)
	}
}
