# Task: Implement BCP-004 Receiver and Sender Capabilities

## Summary
BCP-004 defines how IS-04 Receivers (BCP-004-01) and Senders (BCP-004-02) express parametric constraints on the types of streams they can consume and produce respectively. Currently, both Receiver and Sender resources have empty `caps` objects. This task requires implementing the `constraint_sets` attribute with proper Parameter Constraints supporting types (string, integer, number, boolean, rational), Constraint Keywords (enum, minimum, maximum), and metadata (label, preference, enabled). Controllers need this information to determine whether a Receiver can handle a specific Sender's stream before attempting a connection, and whether a Sender can produce streams matching specific parameters.

## Priority
Major (High)

## Referenced Files
- `internal/infrastructure/nmos_controller.go`
- `internal/module/gateway.go`

## Referenced Specifications
- [AMWA BCP-004-01: Receiver Capabilities](https://specs.amwa.tv/bcp-004-01/releases/v1.0.0/docs/1.0._Receiver_Capabilities.html)
- [AMWA BCP-004-02: Sender Capabilities](https://specs.amwa.tv/bcp-004-02/releases/v1.0.0/docs/Sender_Capabilities.html)
- [NMOS Parameter Registers: Capabilities](https://specs.amwa.tv/nmos-parameter-registers/branches/main/capabilities)
- [AMWA IS-04 Node API: Receivers](https://specs.amwa.tv/is-04/latest/docs/Node_API.html#receivers)
- [AMWA IS-04 Node API: Senders](https://specs.amwa.tv/is-04/latest/docs/Node_API.html#senders)

## Complexity
Medium-High (Requires defining constraint set data structures, implementing parameter constraint validation, integrating with existing Receiver and Sender resources, and updating the `caps.version` attribute when capabilities change)