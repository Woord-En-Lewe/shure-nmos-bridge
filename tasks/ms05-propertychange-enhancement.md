# Task: Enhance MS-05 PropertyChanged Event Support

## Summary
MS-05-02 defines PropertyChanged event (1e1) with comprehensive change tracking for both scalar and sequence properties. Current implementation in BaseNcObject supports basic PropertyChanged notifications but lacks the sequenceItemIndex field and proper NcPropertyChangeType enum support, which are required for sequence property mutations.

## Priority
Medium

## Referenced Files
- `internal/infrastructure/ncp_objects.go` (lines 57-62, 314-320)
- `internal/infrastructure/ncp_types.go` (lines 86-91)

## Referenced Specifications
- [AMWA MS-05-02: NcPropertyChangedEventData](https://specs.amwa.tv/ms-05-02/branches/v1.0.x/docs/Framework.html#ncpropertychangedeventdata)
- [AMWA MS-05-02: NcPropertyChangeType](https://specs.amwa.tv/ms-05-02/branches/v1.0.x/docs/Framework.html#ncpropertychangetype)

## Complexity
Low (Enhancing existing event data structure)

## Required Changes

### 1. Update PropertyChangedEventData Structure
Current structure is missing `sequenceItemIndex`:
```go
type PropertyChangedEventData struct {
    PropertyID      NCPPropertyID      `json:"propertyId"`
    ChangeType      int                `json:"changeType"` // Should use NcPropertyChangeType enum
    Value           interface{}        `json:"value"`
    // MISSING: sequenceItemIndex - required for sequence property changes
}
```

### 2. Add NcPropertyChangeType Enum
Define proper enum values matching MS-05-02 spec:
- `ValueChanged` (0): Current value changed for scalar or sequence property
- `SequenceItemAdded` (1): Sequence item added
- `SequenceItemChanged` (2): Sequence item changed
- `SequenceItemRemoved` (3): Sequence item removed

### 3. Updated Structure Should Be
```go
type PropertyChangedEventData struct {
    PropertyID          NCPPropertyID  `json:"propertyId"`
    ChangeType          int           `json:"changeType"` // 0=ValueChanged,1=SequenceItemAdded, 2=SequenceItemChanged, 3=SequenceItemRemoved
    Value               interface{}   `json:"value"`
    SequenceItemIndex   *int          `json:"sequenceItemIndex,omitempty"` // Index for sequence changes
}
```

### 4. Add Helper Methods to BaseNcObject
Create convenience methods for triggering different change types:
- `NotifyPropertyChanged(id NCPPropertyID, value interface{})` - for scalar changes
- `NotifySequenceItemAdded(id NCPPropertyID, index int, value interface{})` - for sequence additions
- `NotifySequenceItemChanged(id NCPPropertyID, index int, value interface{})` - for sequence modifications
- `NotifySequenceItemRemoved(id NCPPropertyID, index int)` - for sequence removals

### 5. Update NcWorker Example
Lines 314-320 in ncp_objects.go currently call Notify with ChangeType=0. Should use proper enum constant.

## Implementation Notes
- sequenceItemIndex is only meaningful when ChangeType is 1, 2, or 3
- For ValueChanged (0), sequenceItemIndex should be omitted fromJSON
- This enhancement is critical for IS-12 implementations that support sequence properties
- Must be backwards compatible with existing code using ChangeType=0