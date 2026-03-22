# NCA Generic Model

## Overview

This module provides a generic implementation of the NMOS Control Architecture (NCA) as defined in [MS-05-01](../../library/NMOS/MS/MS-05-01_NMOS_Control_Architecture.md) and [MS-05-02](../../library/NMOS/MS/MS-05-02_NMOS_Control_Framework.md).

## Architecture

The NCA model is organized around:

- **NcObject**: Base abstract class for all control objects
- **NcBlock**: Container class that groups control objects
- **NcWorker**: Base class for control/monitoring features
- **NcManager**: Base class for device-wide managers
- **Device Model**: Root block containing all control objects

## Class Hierarchy (MS-05-02)

```
NcObject (1)
├── NcBlock (1.1)
│   ├── 2p1: enabled (readonly boolean)
│   ├── 2p2: members (readonly sequence)
│   ├── 2m1: GetMemberDescriptors(recurse) → descriptors
│   ├── 2m2: FindMembersByPath(path) → descriptors
│   ├── 2m3: FindMembersByRole(role, caseSensitive, matchWhole, recurse) → descriptors
│   └── 2m4: FindMembersByClassId(classId, includeDerived, recurse) → descriptors
├── NcWorker (1.2)
│   └── 2p1: enabled (boolean)
└── NcManager (1.3)
    ├── NcDeviceManager (1.3.1)
    │   ├── 3p1: ncVersion (readonly VersionCode)
    │   ├── 3p2: manufacturer (readonly Manufacturer)
    │   ├── 3p3: product (readonly Product)
    │   ├── 3p4: serialNumber (readonly string)
    │   ├── 3p5: userInventoryCode (string, nullable)
    │   ├── 3p6: deviceName (string, nullable)
    │   ├── 3p7: deviceRole (string, nullable)
    │   ├── 3p8: operationalState (readonly NcDeviceOperationalState)
    │   ├── 3p9: resetCause (readonly NcResetCause)
    │   └── 3p10: message (readonly string, nullable)
    └── NcClassManager (1.3.2)
        ├── 3p1: controlClasses (readonly sequence)
        ├── 3p2: datatypes (readonly sequence)
        ├── 3m1: GetControlClass(classId, includeInherited) → ClassDescriptor
        └── 3m2: GetDatatype(name, includeInherited) → DatatypeDescriptor
```

## NcObject Base (1)

All NCA objects inherit from NcObject:

### Properties (1p*)
- **1p1**: classId (readonly NcClassId)
- **1p2**: oid (readonly NcOid)
- **1p3**: constantOid (readonly boolean)
- **1p4**: owner (readonly NcOid, nullable)
- **1p5**: role (readonly string)
- **1p6**: userLabel (string, nullable)
- **1p7**: touchpoints (readonly sequence<NcTouchpoint>, nullable)
- **1p8**: runtimePropertyConstraints (readonly sequence, nullable)

### Methods (1m*)
- **1m1**: Get(propertyId) → NcMethodResultPropertyValue
- **1m2**: Set(propertyId, value) → NcMethodResult
- **1m3**: GetSequenceItem(propertyId, index) → value
- **1m4**: SetSequenceItem(propertyId, index, value)
- **1m5**: AddSequenceItem(propertyId, value) → index
- **1m6**: RemoveSequenceItem(propertyId, index)
- **1m7**: GetSequenceLength(propertyId) → length

### Events (1e*)
- **1e1**: PropertyChanged(NcPropertyChangedEventData)

## Status Codes (NcMethodStatus)

Per MS-05-02:

| Status | Value | Description |
|--------|-------|-------------|
| Ok | 200 | Success |
| PropertyDeprecated | 298 | Property deprecated |
| MethodDeprecated | 299 | Method deprecated |
| BadCommandFormat | 400 | Malformed command |
| Unauthorized | 401 | Not authorized |
| BadOid | 404 | Object not found |
| Readonly | 405 | Read-only property |
| InvalidRequest | 406 | Invalid in current context |
| Conflict | 409 | State conflict |
| BufferOverflow | 413 | Payload too large |
| IndexOutOfBounds | 414 | Sequence index invalid |
| ParameterError | 417 | Invalid parameter |
| Locked | 423 | Object locked |
| DeviceError | 500 | Internal error |
| MethodNotImplemented | 501 | Method not implemented |
| PropertyNotImplemented | 502 | Property not implemented |
| NotReady | 503 | Device not ready |
| Timeout | 504 | Operation timed out |

## Usage

### Creating a Device Model

```go
// Create root block (OID 1, fixed role "root")
root := nca.NewBlock(1, nil, "root", "Root Block")

// Create resolver to find objects by OID
root.SetResolver(func(oid int) nca.Object {
    switch oid {
    case 2: return deviceMgr
    case 3: return classMgr
    case 101: return worker
    default: return nil
    }
})

// Add device manager
deviceMgr := nca.NewDeviceManager(2, nca.PtrInt(1))
root.AddItem(2)

// Add class manager
classMgr := nca.NewClassManager(3, nca.PtrInt(1))
root.AddItem(3)

// Add worker
worker := nca.NewWorker(101, []int{1, 2, 1, 1}, nca.PtrInt(1), "gain", "Audio Gain")
root.AddItem(101)
```

### Property Access via Get/Set

```go
// Get a property using the generic Get method
result := worker.InvokeMethod(nca.MethodID{Level: 1, Index: 1}, 
    json.RawMessage(`{"propertyId":{"level":3,"index":1}}`))

// Set a property using the generic Set method
result := worker.InvokeMethod(nca.MethodID{Level: 1, Index: 2},
    json.RawMessage(`{"propertyId":{"level":3,"index":1},"value":-6.0}`))
```

### Finding Members

```go
// Find by path
result := block.InvokeMethod(nca.MethodID{Level: 2, Index: 2},
    json.RawMessage(`{"path":["DeviceManager"]}`))

// Find by role (case-insensitive, partial match, recursive)
result := block.InvokeMethod(nca.MethodID{Level: 2, Index: 3},
    json.RawMessage(`{"role":"gain","caseSensitive":false,"matchWholeString":false,"recurse":true}`))

// Find by class ID (including derived classes)
result := block.InvokeMethod(nca.MethodID{Level: 2, Index: 4},
    json.RawMessage(`{"classId":[1,2],"includeDerived":true,"recurse":true}`))
```

### Property Change Notifications

```go
worker.SetNotifyCallback(func(oid int, eventID nca.EventID, eventData nca.PropertyChangedData) {
    fmt.Printf("Property changed: OID=%d, PropertyID=%v, Value=%v\n", 
        oid, eventData.PropertyID, eventData.Value)
})

// Setting value triggers notification
worker.SetValue(-6.0) // → callback invoked
```

## Touchpoints

Touchpoints link NCA objects to NMOS IS-04 resources:

```go
worker.Touchpoints = []nca.Touchpoint{
    {
        ContextNamespace: "x-nmos",
        Resource: nca.TouchpointResourceNmos{
            UUID: "abc123-def456-...",
        },
    },
}
```

## IS-07 Integration

Metered parameters emit `PropertyChanged` events for IS-07 websocket broadcasting. Subscribe to receive notifications without polling.

## IS-12 Integration

The model implements IS-12 NCP protocol:
- Objects expose Get/Set methods for property access
- Sequence methods handle collection operations
- Class discovery via NcClassManager

## Implementation Notes

1. **OID Allocation**: OIDs are allocated at device initialization and remain constant until reboot
2. **Role Uniqueness**: Roles must be unique within a block but can repeat in different blocks
3. **ConstantOID**: Objects with `ConstantOID=true` have stable OIDs across reboots
4. **Search Methods**: Block search methods support recursive traversal for discovering nested objects
