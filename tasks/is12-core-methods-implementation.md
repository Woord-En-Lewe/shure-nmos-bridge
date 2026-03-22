# Task: Implement IS-12 (NCP) Core Sequence Methods

## Summary
The IS-12 specification mandates standard methods for sequence manipulation (1m3 to 1m7) on core control objects (`NcObject`). These are currently missing, though property Get/Set (1m1, 1m2) are present.

## Priority
Medium

## Referenced Files
- `internal/infrastructure/ncp_objects.go`
- `internal/infrastructure/ncp_types.go`

## Referenced Specifications
- [AMWA MS-05-02: NcObject Methods](https://specs.amwa.tv/ms-05-02/branches/v1.0.x/docs/Framework.html#ncobject)

## Complexity
Medium (Requires sequence handling logic, proper notifications, and error handling)

## Related Tasks
- **ms05-propertychange-enhancement.md** - Required for proper sequence change notifications
- **is12-getsequenceitem-enhancement.md** - Detailed sequence method implementation

## Missing Methods
- GetSequenceItem (1m3) - Get item from sequence property
- SetSequenceItem (1m4) - Set item in sequence property
- AddSequenceItem (1m5) - Add item to sequence property
- RemoveSequenceItem (1m6) - Remove item from sequence property
- GetSequenceLength (1m7) - Get sequence property length

All methods must return NcMethodStatus-compliant results and trigger appropriate PropertyChanged events.
