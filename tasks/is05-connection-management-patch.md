# Task: Implement IS-05 Staged Parameter Support (PATCH)

## Summary
The IS-05 Connection API must support `PATCH` requests to the `/staged` endpoint of Senders and Receivers to configure transport parameters for future activation. Currently, the `handleConnectionSenders` implementation is read-only and ignores incoming data.

## Priority
Critical (High)

## Referenced Files
- `internal/infrastructure/nmos_controller.go`

## Referenced Specifications
- [AMWA IS-05 Behaviour: Re-Activating Senders & Receivers](https://specs.amwa.tv/is-05/latest/docs/Behaviour.html#re-activating-senders--receivers)

## Complexity
High (Requires dynamic schema validation and state management for each Sender/Receiver resource)
