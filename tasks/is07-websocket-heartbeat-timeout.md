# Task: Implement IS-07 WebSocket Heartbeat Timeout Enforcement

## Summary
The IS-07 WebSocket transport specification requires the server to prune client subscriptions and close connections if a client heartbeat (`health` command) is not received within 12 seconds. Currently, the server responds to heartbeats but never enforces a timeout.

## Priority
Major (High)

## Referenced Files
- `internal/infrastructure/nmos_controller.go`

## Referenced Specifications
- [AMWA IS-07 Transport - Websocket: Heartbeats](https://specs.amwa.tv/is-07/latest/docs/Transport_-_Websocket.html#heartbeats)

## Complexity
Medium (Requires per-connection timers and cleanup logic)
