# Migration Plan: Remove SAMPLE Values from IS-12 NCP

## Overview

All SAMPLE values across **all model families** (Axient Digital, ULXD, QLXD, SLXD, SLXD+) should be exposed only via IS-07 event streams, not via IS-12 NCP workers.

## Rationale

- SAMPLE messages are device-initiated pushes at configured METER_RATE
- REP messages are client-initiated GET requests with responses
- Metered values (AudioLevel, RSSI, RF_LEVEL, etc.) only arrive via SAMPLE - they are never returned via REP
- These values are already correctly broadcast via IS-07
- The `updateMeteredWorkersFromSample()` function redundantly updates NCP workers for a subset of these values
- This redundancy should be removed

## Values Already Broadcast via IS-07 (keep as-is)

**Axient Digital**: CHAN_QUALITY, AUDIO_LED_BITMAP, AUDIO_LEVEL_PEAK, AUDIO_LEVEL_RMS, ANTENNA_STATUS, RF_LED_BITMAP_A/B/C/D/F1/F2, RF_RSSI_A/B/C/D/F1/F2

**ULXD/QLXD**: ANTENNA_A_ACTIVE, ANTENNA_B_ACTIVE, RF_LEVEL, AUDIO_LEVEL_PEAK, AUDIO_LEVEL_RMS

**SLXD/SLXD+**: AUDIO_LEVEL_PEAK, AUDIO_LEVEL_RMS, RF_RSSI

## What to Remove

### 1. Remove NCP Class Descriptors (`internal/module/gateway.go`)

| Lines | Class Name | Class ID |
|-------|------------|----------|
| 361-368 | AudioLevelWorker | [1,2,1,17] |
| 369-376 | RSSIWorker | [1,2,1,18] |
| 553-562 | AntennaStatusWorker | [1,2,1,41] |
| 564-571 | RFLevelWorker | [1,2,1,42] |
| 572-579 | AudioLevelMeterWorker | [1,2,1,43] |

### 2. Remove WorkerType Constants (`internal/infrastructure/ncp_workers.go`)

From enum (lines 7-44):
- `WorkerTypeAudioLevel WorkerType = 7`
- `WorkerTypeRSSI WorkerType = 8`
- `WorkerTypeAudioLevelMeter WorkerType = 33`
- `WorkerTypeAntStatus WorkerType = 31`
- `WorkerTypeRFLevel WorkerType = 32`

### 3. Remove Class ID Mappings (`internal/infrastructure/ncp_workers.go`)

**AxientDigital map**:
- Remove `WorkerTypeAudioLevel: [1,2,1,17]`
- Remove `WorkerTypeRSSI: [1,2,1,18]`

**ULXD map**:
- Remove `WorkerTypeAudioLevel: [1,2,1,17]`
- Remove `WorkerTypeRSSI: [1,2,1,18]`

**QLXD map**:
- Remove `WorkerTypeAntStatus: [1,2,1,41]`
- Remove `WorkerTypeRFLevel: [1,2,1,42]`
- Remove `WorkerTypeAudioLevelMeter: [1,2,1,43]`

**SLXD map**:
- Remove `WorkerTypeAudioLevel: [1,2,1,17]`
- Remove `WorkerTypeRSSI: [1,2,1,18]`

**SLXDPlus map**:
- Remove `WorkerTypeAudioLevel: [1,2,1,17]`
- Remove `WorkerTypeRSSI: [1,2,1,18]`

### 4. Remove ChannelWorkersConfig Entries (`internal/infrastructure/ncp_workers.go`)

**AxientDigital** (~lines 159-175): Remove AudioLevel and RSSI entries

**ULXD** (~lines 177-184): Remove AudioLevel and RSSI entries

**QLXD** (~lines 178-186): Remove AntennaStatus, RFLevel, AudioLevelMeter entries

**SLXD** (~lines 188-198): Remove AudioLevel and RSSI entries

**SLXDPlus** (~lines 200-210): Remove AudioLevel and RSSI entries

### 5. Remove Functions and Call Site (`internal/module/gateway.go`)

| Location | Action |
|----------|--------|
| Line 1113 | Remove call to `g.updateMeteredWorkersFromSample(info, report)` |
| Lines 1427-1453 | Delete `updateMeteredWorkersFromSample()` function |
| Lines 1455-1469 | Delete `updateWorkerValue()` function |

## Files Modified

- `internal/module/gateway.go` (~50 lines removed)
- `internal/infrastructure/ncp_workers.go` (~60 lines removed)

## Impact

- SAMPLE values will still be broadcast via IS-07 (no change)
- NCP tree will no longer expose metered values as workers
- Clients must use IS-07 WebSocket subscriptions to receive metered values
