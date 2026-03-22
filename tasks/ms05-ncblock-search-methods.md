# Task: Implement MS-05 NcBlock Search Methods

## Summary
MS-05-02 defines four methods for NcBlock (2m1-2m4) to discover and search for members. Currently only GetMemberDescriptors (2m1) is partially implemented. Missing are FindMembersByPath, FindMembersByRole, and FindMembersByClassId, which are essential for device model discovery and navigation.

## Priority
Major (High)

## Referenced Files
- `internal/infrastructure/ncp_objects.go` (lines 136-155)
- `internal/infrastructure/ncp_types.go`

## Referenced Specifications
- [AMWA MS-05-02: NcBlock](https://specs.amwa.tv/ms-05-02/branches/v1.0.x/docs/Framework.html#ncblock)
- [AMWA MS-05-02: Blocks](https://specs.amwa.tv/ms-05-02/branches/v1.0.x/docs/Blocks.html)

## Complexity
Medium (Requires recursive search through block hierarchy and proper descriptor collection)

## Missing Methods (Level 2)
1. **GetMemberDescriptors (2m1)** - PARTIALLY IMPLEMENTED
   - Parameters: `recurse` (NcBoolean)
   - Returns: NcMethodResultBlockMemberDescriptors
   - Current implementation missing recurse parameter support
2. **FindMembersByPath (2m2)** - NOT IMPLEMENTED
   - Parameters: `path` (NcRolePath - sequence of role strings)
   - Returns: NcMethodResultBlockMemberDescriptors
   - Finds members matching a relative role path
3. **FindMembersByRole (2m3)** - NOT IMPLEMENTED
   - Parameters: `role` (NcString), `caseSensitive` (NcBoolean), `matchWholeString` (NcBoolean), `recurse` (NcBoolean)
   - Returns: NcMethodResultBlockMemberDescriptors
   - Searches for members by role name with flexible matching
4. **FindMembersByClassId (2m4)** - NOT IMPLEMENTED
   - Parameters: `classId` (NcClassId), `includeDerived` (NcBoolean), `recurse` (NcBoolean)
   - Returns: NcMethodResultBlockMemberDescriptors
   - Searches for members by class ID with optional inheritance support

## Implementation Notes
- All methods should support recursive search through nested blocks
- FindMembersByPath must not include the role of the block targeted by oid in the path parameter
- IsDerivedFrom method already exists in NcClassManager for includeDerived support
- NcRolePath is defined as `sequence<NcString>` - an ordered list of roles