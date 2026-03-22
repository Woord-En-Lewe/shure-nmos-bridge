# Task: Implement IS-07 WebSocket Heartbeat Timeout Enforcement

## Summary
The IS-07 WebSocket transport specification requires the server to prune client subscriptions and close connections if a client heartbeat (`health` command) is not received within 12 seconds. 

## Status: COMPLETED

The heartbeat timeout enforcement has been implemented in `internal/infrastructure/nmos_controller.go`:

1. Added `clientLastHealth map[*websocket.Conn]time.Time` field to track last health timestamp per client
2. Updated health command handler to record timestamp on each received `health` command
3. Added `checkIS07Heartbeats()` goroutine that runs every 2 seconds to check for stale connections
4. Connections with no health received for >12 seconds are closed with a log warning
5. Cleanup of `clientLastHealth` on client disconnect

## Priority
Major (High)

## Referenced Files
- `internal/infrastructure/nmos_controller.go`

## Referenced Specifications
- [AMWA IS-07 Transport - Websocket: Heartbeats](https://specs.amwa.tv/is-07/latest/docs/Transport_-_Websocket.html#heartbeats)

## Complexity
Medium (Requires per-connection timers and cleanup logic)
