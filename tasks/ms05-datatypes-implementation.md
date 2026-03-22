# Task: Implement MS-05-02 Core Datatypes

## Summary
MS-05-02 Framework defines numerous datatypes required for control class properties and method results. Many of these are currently missing from ncp_types.go, limiting the completeness of the MS-05 implementation. These datatypes enable proper serialization, validation, and type safety for the control protocol.

## Priority
Major (High)

## Referenced Files
- `internal/infrastructure/ncp_types.go`

## Referenced Specifications
- [AMWA MS-05-02: Framework - Datatypes](https://specs.amwa.tv/ms-05-02/branches/v1.0.x/docs/Framework.html#datatypes)

## Complexity
Low (Data structure definitions, but many types to implement)

## Required Primitive Datatypes
Already partially implemented or primitives in Go:
- NcBoolean (bool)
- NcInt16, NcInt32, NcInt64 (int16, int32, int64)
- NcUint16, NcUint32, NcUint64 (uint16, uint32, uint64)
- NcFloat32, NcFloat64 (float32, float64)
- NcString (string) - UTF-8
- NcOid (uint32) - Object identifier
- NcId (uint32) - Generic identity handler
- NcName (string) - Programmatically significant name (alphanumerics + underscore)
- NcUri (string) - URI
- NcUuid (string) - UUID string
- NcRegex (string) - Regex pattern
- NcTimeInterval (int64) - Time interval in nanoseconds
- NcVersionCode (string) - Semantic version "vMajor.Minor.Patch"

## Required Struct Datatypes
Missing or incomplete:
1. **NcManufacturer**
   - `name` (NcString): Manufacturer name
   - `organizationId` (NcOrganizationId?, nullable): IEEE OUI or CID
   - `website` (NcUri?, nullable): URL
   
2. **NcProduct**
   - `name`, `key`, `revisionLevel` (NcString): Product identifiers
   - `brandName`, `description` (NcString?, nullable)
   - `uuid` (NcUuid?, nullable): Product UUID (not instance UUID)
   
3. **NcDeviceOperationalState**
   - `generic` (NcDeviceGenericState): Unknown/NormalOperation/Initializing/Updating/LicensingError/InternalError
   - `deviceSpecificDetails` (NcString?, nullable)
   
4. **NcMethodResult Variants** (only NcMethodResult and NcMethodResultPropertyValue exist)
   - NcMethodResultBlockMemberDescriptors (for block search methods)
   - NcMethodResultClassDescriptor (for GetControlClass)
   - NcMethodResultDatatypeDescriptor (for GetDatatype)
   - NcMethodResultError (for error responses)
   - NcMethodResultId (for AddSequenceItem)
   - NcMethodResultLength (for GetSequenceLength)

5. **NcPropertyChangedEventData** - PARTIALLY IMPLEMENTED
   - Missing: `sequenceItemIndex` (NcId?, nullable) for sequence property changes

## Required Enum Datatypes
1. **NcDeviceGenericState**: Unknown(0), NormalOperation(1), Initializing(2), Updating(3), LicensingError(4), InternalError(5)
2. **NcResetCause**: Unknown(0), PowerOn(1), InternalError(2), Upgrade(3), ControllerRequest(4), ManualReset(5)
3. **NcPropertyChangeType**: ValueChanged(0), SequenceItemAdded(1), SequenceItemChanged(2), SequenceItemRemoved(3)
4. **NcMethodStatus**: Ok(200), PropertyDeprecated(298), MethodDeprecated(299), BadCommandFormat(400), Unauthorized(401), BadOid(404), Readonly(405), InvalidRequest(406), Conflict(409), BufferOverflow(413), IndexOutOfBounds(414), ParameterError(417), Locked(423), DeviceError(500), MethodNotImplemented(501), PropertyNotImplemented(502), NotReady(503), Timeout(504)
5. **NcDatatypeType**: Primitive(0), Typedef(1), Struct(2), Enum(3)

## Implementation Notes
- All types should have proper JSON tags for serialization
- NcMethodStatus uses HTTP-like status codes but should be distinct type
- NcOrganizationId is negated IEEE OUI/CID integer
- Method result types enable proper error handling and status codes in IS-12 protocol