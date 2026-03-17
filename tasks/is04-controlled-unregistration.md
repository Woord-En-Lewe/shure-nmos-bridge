# Task: Implement IS-04 Controlled Unregistration

## Summary
When the gateway shuts down, it should explicitly unregister its Node and all associated resources (Devices, Senders, Receivers, etc.) by making HTTP `DELETE` calls to the Registration API. Current implementation stops the Node API server but doesn't notify the registry.

## Priority
Medium

## Referenced Files
- `internal/infrastructure/nmos_controller.go`
- `internal/module/gateway.go`

## Referenced Specifications
- [AMWA IS-04 Behaviour: Registration - Controlled Unregistration](https://specs.amwa.tv/is-04/latest/docs/Behaviour_-_Registration.html#controlled-unregistration)

## Complexity
Medium (Requires tracking all registered resource IDs and performing sequential deletions)
