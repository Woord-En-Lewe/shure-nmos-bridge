# Task: Implement TLS Support (BCP-003-01)

## Summary
To comply with AMWA BCP-003-01, all NMOS APIs (Node, Registration, Connection, Events, and NCP) should be served over HTTPS/TLS. Currently, all APIs are hardcoded to HTTP.

## Priority
Major (High for Production)

## Referenced Files
- `internal/infrastructure/nmos_controller.go`
- `cmd/gateway/main.go`

## Referenced Specifications
- [AMWA BCP-003-01: Secure Communication in NMOS Systems](https://specs.amwa.tv/bcp-003-01/latest/docs/Overview.html)

## Complexity
High (Requires certificate management, updated mDNS advertisement, and client-side support for HTTPS)
