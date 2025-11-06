# Hetzner DNS for `libdns`

[![godoc reference](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/github.com/libdns/hetzner)

This package implements the [libdns](https://github.com/libdns/libdns) interfaces for the Hetzner DNS APIs.

- The `v1` version of this package (`github.com/libdns/hetzner`) implements the [deprecated Hetzner DNS Console API](https://dns.hetzner.com/api-docs) (zones in the DNS Console)
- The `v2` version of this package (`github.com/libdns/hetzner/v2`) implements the [new Hetzner DNS Cloud API](https://docs.hetzner.cloud/reference/cloud#dns) (zones in the Hetzner Console)

## Authenticating

To authenticate, you need to supply a Hetzner Cloud [API token](https://docs.hetzner.cloud/reference/cloud#getting-started).

## Example

See [examples](examples) and [provider_test.go](provider_test.go).

## License

MIT
