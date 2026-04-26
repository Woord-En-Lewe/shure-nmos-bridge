# IS-07 Eager Registration Plan

## Problem Statement
Current IS-07 streams are only registered lazily when a device sends REP or SAMPLE data. The requirement is to register IS-07 streams per device when a device is discovered and connected to.

## Additional Requirement
Only SAMPLE commands should register IS-07 resources, not REP commands.

## Summary of SAMPLE Parameters by Model Family

| Model Family | SAMPLE Audio Params | RF Params |
|--------------|---------------------| ----------|
| **Axient Digital** | `AUDIO_LEVEL_PEAK`, `AUDIO_LEVEL_RMS` | `RF_RSSI_A/B/C/D`, `RF_LED_BITMAP_A/B/C/D`, etc. |
| **ULX-D / QLX-D** | `AUDIO_LVL` (single value) | `RF_LEVEL` (single value) |
| **SLX-D / SLX-D+** | `AUDIO_LEVEL_PEAK`, `AUDIO_LEVEL_RMS` | `RF_RSSI` (single value) |

## Changes Required

### 1. Add `GetIS07MeteredParams(modelFamily ShureModelFamily) []string` in `shure_commands.go`

Returns model-specific metered params that appear in SAMPLE ALL:

```go
func GetIS07MeteredParams(modelFamily ShureModelFamily) []string {
    switch modelFamily {
    case ModelFamilyAxientDigital:
        return []string{
            "CHAN_QUALITY", "AUDIO_LED_BITMAP",
            "AUDIO_LEVEL_PEAK", "AUDIO_LEVEL_RMS",
            "ANTENNA_A_ACTIVE", "ANTENNA_B_ACTIVE",
            "RF_LED_BITMAP_A", "RF_RSSI_A",
            "RF_LED_BITMAP_B", "RF_RSSI_B",
            "RF_LED_BITMAP_C", "RF_RSSI_C",
            "RF_LED_BITMAP_D", "RF_RSSI_D",
            "RF_LED_BITMAP_F1", "RF_RSSI_F1",
            "RF_LED_BITMAP_F2", "RF_RSSI_F2",
        }
    case ModelFamilyULXD, ModelFamilyQLXD:
        return []string{
            "ANTENNA_A_ACTIVE", "ANTENNA_B_ACTIVE",
            "RF_LEVEL", "AUDIO_LVL",
        }
    case ModelFamilySLXD, ModelFamilySLXDPlus:
        return []string{
            "AUDIO_LEVEL_PEAK", "AUDIO_LEVEL_RMS",
            "RF_RSSI",
        }
    default:
        return nil
    }
}
```

### 2. Add `ensureIS07ResourcesForChannel()` in `gateway.go`

```go
func (g *gatewayImpl) ensureIS07ResourcesForChannel(info *shureDeviceInfo, channel int, deviceID string, deviceInstance string) {
    params := infrastructure.GetIS07MeteredParams(info.modelFamily)
    for _, param := range params {
        g.ensureIS07Resources(info, channel, param, deviceID, deviceInstance)
    }
}
```

### 3. Modify `addShureController()` in `gateway.go`

After METER_RATE setup (around line 270), call eager registration for all channels:

```go
// After METER_RATE setup loop
maxChannels := infrastructure.MaxChannelsFromModel(dev.Instance)
for ch := 1; ch <= maxChannels; ch++ {
    g.ensureIS07ResourcesForChannel(info, ch, deviceID, dev.Instance)
}
```

### 4. Modify REP handling in `gateway.go`

REMOVE `ensureIS07Resources()` call from REP handling (lines 1062). REP should only broadcast if IS-07 resources are already registered:

```go
// IS-07: Metered params broadcast events (only if already registered)
if infrastructure.IsMeteredParam(report.Param) {
    deviceID := info.nmosDeviceIDs[0]
    deviceInstance := info.deviceInstance

    // Check if already registered before broadcasting
    if sID, ok := info.sourceIDs[report.Channel][report.Param]; ok {
        g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel][report.Param], getNMOSEventType(report.Param), report.Value)
    }
    // ... NCP update code remains
}
```

### 5. Fix `handleULXDSampleEvents()` in `gateway.go`

Change `AUDIO_LEVEL_PEAK` and `AUDIO_LEVEL_RMS` to `AUDIO_LVL`:

```go
g.ensureIS07Resources(info, channel, "AUDIO_LVL", deviceID, deviceInstance)
if sID, ok := info.sourceIDs[channel]["AUDIO_LVL"]; ok {
    g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["AUDIO_LVL"], "number", sample.AudioLevelDBFS())
}
```

### 6. Update `getNMOSEventType()` in `gateway.go`

Ensure `AUDIO_LVL` returns `"number"`:

```go
case "AUDIO_LVL":
    return "number"
```

### 7. Ensure `IsMeteredParam()` includes `AUDIO_LVL` for ULX-D/QLX-D

Check that `AUDIO_LVL` is in the metered params list in `shure_commands.go`.

## Files Affected

- `internal/infrastructure/shure_commands.go`
  - Add `GetIS07MeteredParams()` function

- `internal/module/gateway.go`
  - Add `ensureIS07ResourcesForChannel()` function
  - Modify `addShureController()` to call eager registration
  - Modify REP handling to NOT call `ensureIS07Resources()`
  - Fix `handleULXDSampleEvents()` to use `AUDIO_LVL`
  - Update `getNMOSEventType()` to handle `AUDIO_LVL`
