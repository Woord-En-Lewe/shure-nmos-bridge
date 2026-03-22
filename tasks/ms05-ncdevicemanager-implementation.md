# Task: Implement MS-05 NcDeviceManager

## Summary
MS-05-02 requires a Device Manager singleton (NcDeviceManager, class 1.3.1, fixed role "DeviceManager") as a mandatory component in the root block. It provides device identification, operational state tracking, and manufacturer/product information. Currently completely missing from the implementation.

## Priority
Major (High)

## Referenced Files
- `internal/infrastructure/ncp_objects.go`
- `internal/infrastructure/nmos_controller.go`

## Referenced Specifications
- [AMWA MS-05-02: NcDeviceManager](https://specs.amwa.tv/ms-05-02/branches/v1.0.x/docs/Framework.html#ncdevicemanager)
- [AMWA MS-05-02: Managers](https://specs.amwa.tv/ms-05-02/branches/v1.0.x/docs/Managers.html)

## Complexity
Medium (Standard singleton class with required properties and MS version tracking)

## Required Properties (Level 3)
1. `ncVersion` (3p1, readonly): [NcVersionCode](#ncversioncode) - Version ofMS-05-02 this device uses
2. `manufacturer` (3p2, readonly): [NcManufacturer](#ncmanufacturer) - Manufacturer descriptor
3. `product` (3p3, readonly): [NcProduct](#ncproduct) - Product descriptor
4. `serialNumber` (3p4, readonly): NcString - Serial number
5. `userInventoryCode` (3p5, nullable): NcString? - Asset tracking ID (user specified)
6. `deviceName` (3p6, nullable): NcString? - Instance name (not product name)
7. `deviceRole` (3p7, nullable): NcString? - Role in application
8. `operationalState` (3p8, readonly): [NcDeviceOperationalState](#ncdeviceoperationalstate) - Device state
9. `resetCause` (3p9, readonly): [NcResetCause](#ncresetcause) - Reason for most recent reset
10. `message` (3p10, readonly, nullable): NcString? - Arbitrary message from device

## Dependencies
- Requires implementing NcManufacturer, NcProduct, NcVersionCode (primitives), NcDeviceOperationalState, NcDeviceGenericState (enum), NcResetCause (enum) datatypes first
- Must be instantiated in root block with OID and fixed role "DeviceManager"