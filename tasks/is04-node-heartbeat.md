# Task: Implement IS-04 Node Heartbeat Loop

## Summary
The NMOS IS-04 specification requires that a Node performs an HTTP `POST` to the `/health/nodes/{nodeId}` endpoint of the Registration API every 5 seconds. Currently, the `nmosController` registers itself once but does not maintain heartbeats, leading to its removal from the registry after 12 seconds.

## Priority
Critical (High)

## Referenced Files
- `internal/infrastructure/nmos_controller.go`
- `internal/module/gateway.go`

## Referenced Specifications
- [AMWA IS-04 Behaviour: Registration - Heartbeating](https://specs.amwa.tv/is-04/latest/docs/Behaviour_-_Registration.html#heartbeating)

## Complexity
Medium (Requires a background goroutine with error handling and retry logic)
