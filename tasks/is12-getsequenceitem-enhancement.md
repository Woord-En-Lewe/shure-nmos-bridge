# Task: Implement IS-12 Sequence Item Methods Enhancement

## Summary
The existing task `is12-core-methods-implementation.md` covers the basic sequence methods (1m3-1m7) but doesn't address the need for enhanced sequence support. This task tracks specific enhancements needed for sequence properties beyond basic Get/Set operations, including proper handling of sequence mutations and notifications.

## Priority
Medium

## Referenced Files
- `internal/infrastructure/ncp_objects.go`
- `internal/infrastructure/ncp_types.go`

## Referenced Specifications
- [AMWA IS-12: Sequence Manipulation](https://specs.amwa.tv/ms-05-02/branches/v1.0.x/docs/NcObject.html#working-with-collections-inside-an-ncobject)

## Complexity
Medium (Requires sequence handling logic and notification updates)

## Enhancement Requirements

### 1. Sequence Property Identification
Need mechanism to identify which properties are sequences:
- Add `IsSequence` flag to property descriptors
- Enable runtime sequence property detection in InvokeMethod
- Support both fixed-size and dynamic sequences

### 2. GetSequenceItem (1m3) - NOT IMPLEMENTED
Parameters:
- `id` (NcPropertyId): Property ID of the sequence
- `index` (NcId): Index of item in the sequence
Returns: `NcMethodResultPropertyValue`

### 3. SetSequenceItem (1m4) - NOT IMPLEMENTED
Parameters:
- `id` (NcPropertyId): Property ID of the sequence
- `index` (NcId): Index of item in the sequence
- `value` (any?): Value to set
Returns: `NcMethodResult`
Should trigger PropertyChanged event with ChangeType=SequenceItemChanged

### 4. AddSequenceItem (1m5) - NOT IMPLEMENTED
Parameters:
- `id` (NcPropertyId): Property ID of the sequence
- `value` (any?): Value to add
Returns: `NcMethodResultId` (contains index where item was added)
Should trigger PropertyChanged event with ChangeType=SequenceItemAdded

### 5. RemoveSequenceItem (1m6) - NOT IMPLEMENTED
Parameters:
- `id` (NcPropertyId): Property ID of the sequence
- `index` (NcId): Index of item to remove
Returns: `NcMethodResult`
Should trigger PropertyChanged event with ChangeType=SequenceItemRemoved

### 6. GetSequenceLength (1m7) - NOT IMPLEMENTED
Parameters:
- `id` (NcPropertyId): Property ID of the sequence
Returns: `NcMethodResultLength` (null if sequence is null, otherwise sequence length)

## Implementation Notes
- All sequence methods should use the enhanced PropertyChangedEventData with sequenceItemIndex
- Error handling should use NcMethodStatus codes: IndexOutOfBounds(414), BadOid(404), PropertyNotImplemented(502)
- Sequences are zero-indexed
- AddSequenceItem should return the index where the item was inserted
- Must handle nil sequences gracefully (GetSequenceLength should return null for nil sequence)
- This builds on MS-05 PropertyChanged enhancement and is a prerequisite for full IS-12 compliance