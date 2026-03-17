# Task: Implement IS-05 Activation Mechanism

## Summary
IS-05 requires that parameters staged at the `/staged` endpoint can be applied through a triggered activation (either immediate or scheduled). The current implementation lacks the logic to apply these parameters to the system or update the `/active` resource.

## Priority
Critical (High)

## Referenced Files
- `internal/infrastructure/nmos_controller.go`
- `internal/module/gateway.go`

## Referenced Specifications
- [AMWA IS-05 Behaviour: Scheduled Activations](https://specs.amwa.tv/is-05/latest/docs/Behaviour.html#scheduled-activations)

## Complexity
Medium-High (Requires integration with the internal message bus to trigger device-level state changes)
