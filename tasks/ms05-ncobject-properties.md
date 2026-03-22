# Task: Implement MS-05 NcObject Optional Properties

## Summary
MS-05-02 defines optional properties on NcObject (1p7, 1p8) for touchpoints and runtime property constraints. These are currently missing from BaseNcObject. Touchpoints enable identity mapping between NCA control objects and NMOS resources (IS-04/IS-05/IS-07/IS-08), essential for integration with existing NMOS infrastructure.

## Priority
Medium

## Referenced Files
- `internal/infrastructure/ncp_objects.go` (lines 10-18, 32-48)
- `internal/infrastructure/ncp_types.go`

## Referenced Specifications
- [AMWA MS-05-02: NcObject](https://specs.amwa.tv/ms-05-02/branches/v1.0.x/docs/NcObject.html)
- [AMWA MS-05-02: Framework - Touchpoints](https://specs.amwa.tv/ms-05-02/branches/v1.0.x/docs/Framework.html#nctouchpoint)
- [AMWA MS-05-01: Identification](https://specs.amwa.tv/ms-05-01/branches/v1.0.x/docs/Identification.html#nca-nmos-identity-mapping)

## Complexity
Low-Medium (Adding properties and touchpoint datatypes)

## Required Properties (Level 1)
1. `touchpoints` (1p7, readonly, nullable): `sequence<NcTouchpoint>?`
   - Enables mapping to NMOS resources (nodes, devices, senders, receivers, etc.)
   - Uses NcTouchpointNmos for general NMOS contexts (IS-04/05/07)
   - Uses NcTouchpointNmosChannelMapping for IS-08 contexts
   
2. `runtimePropertyConstraints` (1p8, readonly, nullable): `sequence<NcPropertyConstraints>?`
   - Provides runtime-specific property constraints
   - Allows device to indicate dynamic limits (e.g., max gain depends on current mode)

## Required Datatypes
1. **NcTouchpoint** (base class)
   - `contextNamespace` (NcString): Must be "x-nmos" or "x-nmos/channelmapping"
2. **NcTouchpointNmos** (extends NcTouchpoint)
   - `resource` (NcTouchpointResourceNmos): Contains resource type and UUID
3. **NcTouchpointNmosChannelMapping** (extends NcTouchpointNmos)
   - `ioId` (NcString): Input or output ID for IS-08
4. **NcTouchpointResource** (base class)
   - `resourceType` (NcString): "node", "device", "source", "flow", "sender", "receiver"
5. **NcTouchpointResourceNmos** (extends NcTouchpointResource)
   - `id` (NcUuid): NMOS resource UUID
6. **NcPropertyConstraints** (base class)
   - `propertyId` (NcPropertyId): Which property is constrained
   - `defaultValue` (any?, nullable): Optional default value
7. **NcPropertyConstraintsNumber** (extends NcPropertyConstraints)
   - `maximum`, `minimum`, `step` (any?, nullable): Range constraints
8. **NcPropertyConstraintsString** (extends NcPropertyConstraints)
   - `maxCharacters` (NcUint32?, nullable)
   - `pattern` (NcRegex?, nullable): Regex pattern constraint

## Implementation Notes
- Both properties are nullable sequences, so GetProperty should return nil when not populated
- Touchpoints should be useful for linking NcWorkers to NMOS senders/receivers for IS-05/IS-07 integration
- Runtime property constraints are optional but enable runtime-aware controllers