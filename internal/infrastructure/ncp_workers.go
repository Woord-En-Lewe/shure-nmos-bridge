package infrastructure

import (
	"fmt"
)

// WorkerType defines the type of worker for proper class ID assignment
type WorkerType int

const (
	WorkerTypeGain               WorkerType = 0
	WorkerTypeMute              WorkerType = 1
	WorkerTypeChannelName        WorkerType = 2
	WorkerTypeFreq              WorkerType = 3
	WorkerTypeGroupChan         WorkerType = 4
	WorkerTypeTX                WorkerType = 5
	WorkerTypeBattery           WorkerType = 6
	WorkerTypeAudioLevel        WorkerType = 7
	WorkerTypeRSSI              WorkerType = 8
	WorkerTypeFWVer             WorkerType = 9
	WorkerTypeDeviceID          WorkerType = 10
	WorkerTypeEncryption        WorkerType = 11
	WorkerTypeMACAddr           WorkerType = 12
	WorkerTypeBattCycle         WorkerType = 13
	WorkerTypeBattRunTime       WorkerType = 14
	WorkerTypeBattTempF         WorkerType = 15
	WorkerTypeBattTempC         WorkerType = 16
	WorkerTypeBattType          WorkerType = 17
	WorkerTypeBattCharge        WorkerType = 18
	WorkerTypeBattHealth        WorkerType = 19
	WorkerTypeBattBars          WorkerType = 20
	WorkerTypeTxType            WorkerType = 21
	WorkerTypeTxOffset          WorkerType = 22
	WorkerTypeTxRFPower         WorkerType = 23
	WorkerTypeTxPwrLock         WorkerType = 24
	WorkerTypeTxMenuLock        WorkerType = 25
	WorkerTypeTxDeviceID        WorkerType = 26
	WorkerTypeTxMuteStatus      WorkerType = 27
	WorkerTypeTxMuteButtonStatus WorkerType = 28
	WorkerTypeTxPowerSource     WorkerType = 29
	WorkerTypeEncryptionWarning WorkerType = 30
	WorkerTypeAntStatus        WorkerType = 31
	WorkerTypeRFLevel          WorkerType = 32
	WorkerTypeAudioLevelMeter  WorkerType = 33
)

// WorkerTypeClassID maps worker type to class ID for a model family
var WorkerTypeClassID = map[ShureModelFamily]map[WorkerType][]int{
	ModelFamilyAxientDigital: {
		WorkerTypeGain:        []int{1, 2, 1, 10},
		WorkerTypeMute:        []int{1, 2, 1, 11},
		WorkerTypeChannelName: []int{1, 2, 1, 12},
		WorkerTypeFreq:        []int{1, 2, 1, 13},
		WorkerTypeGroupChan:   []int{1, 2, 1, 14},
		WorkerTypeTX:          []int{1, 2, 1, 15},
		WorkerTypeBattery:     []int{1, 2, 1, 16},
		WorkerTypeAudioLevel:  []int{1, 2, 1, 17},
		WorkerTypeRSSI:        []int{1, 2, 1, 18},
	},
	ModelFamilyULXD: {
		WorkerTypeGain:        []int{1, 2, 1, 10},
		WorkerTypeMute:        []int{1, 2, 1, 11},
		WorkerTypeChannelName: []int{1, 2, 1, 12},
		WorkerTypeFreq:        []int{1, 2, 1, 13},
		WorkerTypeGroupChan:   []int{1, 2, 1, 14},
		WorkerTypeTX:          []int{1, 2, 1, 15},
		WorkerTypeBattery:     []int{1, 2, 1, 16},
		WorkerTypeAudioLevel:  []int{1, 2, 1, 17},
		WorkerTypeRSSI:        []int{1, 2, 1, 18},
	},
	ModelFamilyQLXD: {
		WorkerTypeGain:               []int{1, 2, 1, 10},
		WorkerTypeMute:              []int{1, 2, 1, 11},
		WorkerTypeChannelName:       []int{1, 2, 1, 12},
		WorkerTypeFreq:              []int{1, 2, 1, 13},
		WorkerTypeGroupChan:         []int{1, 2, 1, 14},
		WorkerTypeBattery:           []int{1, 2, 1, 16},
		WorkerTypeAudioLevel:        []int{1, 2, 1, 17},
		WorkerTypeRSSI:              []int{1, 2, 1, 18},
		WorkerTypeFWVer:             []int{1, 2, 1, 19},
		WorkerTypeDeviceID:          []int{1, 2, 1, 20},
		WorkerTypeEncryption:        []int{1, 2, 1, 21},
		WorkerTypeMACAddr:           []int{1, 2, 1, 22},
		WorkerTypeBattCycle:         []int{1, 2, 1, 23},
		WorkerTypeBattRunTime:       []int{1, 2, 1, 24},
		WorkerTypeBattTempF:         []int{1, 2, 1, 25},
		WorkerTypeBattTempC:         []int{1, 2, 1, 26},
		WorkerTypeBattType:          []int{1, 2, 1, 27},
		WorkerTypeBattCharge:        []int{1, 2, 1, 28},
		WorkerTypeBattHealth:        []int{1, 2, 1, 29},
		WorkerTypeBattBars:          []int{1, 2, 1, 30},
		WorkerTypeTxType:           []int{1, 2, 1, 31},
		WorkerTypeTxOffset:         []int{1, 2, 1, 32},
		WorkerTypeTxRFPower:        []int{1, 2, 1, 33},
		WorkerTypeTxPwrLock:        []int{1, 2, 1, 34},
		WorkerTypeTxMenuLock:       []int{1, 2, 1, 35},
		WorkerTypeTxDeviceID:       []int{1, 2, 1, 36},
		WorkerTypeTxMuteStatus:     []int{1, 2, 1, 37},
		WorkerTypeTxMuteButtonStatus: []int{1, 2, 1, 38},
		WorkerTypeTxPowerSource:    []int{1, 2, 1, 39},
		WorkerTypeEncryptionWarning: []int{1, 2, 1, 40},
		WorkerTypeAntStatus:        []int{1, 2, 1, 41},
		WorkerTypeRFLevel:          []int{1, 2, 1, 42},
		WorkerTypeAudioLevelMeter:  []int{1, 2, 1, 43},
	},
	ModelFamilySLXD: {
		WorkerTypeGain:        []int{1, 2, 1, 10},
		WorkerTypeMute:        []int{1, 2, 1, 11},
		WorkerTypeChannelName: []int{1, 2, 1, 12},
		WorkerTypeFreq:        []int{1, 2, 1, 13},
		WorkerTypeGroupChan:   []int{1, 2, 1, 14},
		WorkerTypeTX:          []int{1, 2, 1, 15},
		WorkerTypeBattery:     []int{1, 2, 1, 16},
		WorkerTypeAudioLevel:  []int{1, 2, 1, 17},
		WorkerTypeRSSI:        []int{1, 2, 1, 18},
	},
	ModelFamilySLXDPlus: {
		WorkerTypeGain:        []int{1, 2, 1, 10},
		WorkerTypeMute:        []int{1, 2, 1, 11},
		WorkerTypeChannelName: []int{1, 2, 1, 12},
		WorkerTypeFreq:        []int{1, 2, 1, 13},
		WorkerTypeGroupChan:   []int{1, 2, 1, 14},
		WorkerTypeTX:          []int{1, 2, 1, 15},
		WorkerTypeBattery:     []int{1, 2, 1, 16},
		WorkerTypeAudioLevel:  []int{1, 2, 1, 17},
		WorkerTypeRSSI:        []int{1, 2, 1, 18},
	},
}

// GetClassIDForWorker returns the class ID for a worker type in a given model family
func GetClassIDForWorker(family ShureModelFamily, wt WorkerType) []int {
	if familyMap, ok := WorkerTypeClassID[family]; ok {
		if classID, ok := familyMap[wt]; ok {
			return classID
		}
	}
	return []int{1, 2, 1, 99} // fallback
}

// ChannelWorkerConfig defines a single parameter worker for a channel
type ChannelWorkerConfig struct {
	WorkerType WorkerType
	Name       string
	ShureParam string // Shure command parameter name (e.g., "AUDIO_GAIN")
	ClassID    []int
	IsReadOnly bool
	TypeName   string
}

// ChannelWorkersConfig returns the list of workers needed per channel for a model family
func ChannelWorkersConfig(family ShureModelFamily) []ChannelWorkerConfig {
	switch family {
	case ModelFamilyAxientDigital:
		return []ChannelWorkerConfig{
			{WorkerTypeGain, "Gain", "AUDIO_GAIN", []int{1, 2, 1, 10}, false, "NcFloat32"},
			{WorkerTypeMute, "Mute", "AUDIO_MUTE", []int{1, 2, 1, 11}, false, "NcBoolean"},
			{WorkerTypeChannelName, "ChannelName", "CH_NAME", []int{1, 2, 1, 12}, false, "NcString"},
			{WorkerTypeFreq, "Frequency", "FREQUENCY", []int{1, 2, 1, 13}, false, "NcString"},
			{WorkerTypeGroupChan, "GroupChannel", "GROUP_CHAN", []int{1, 2, 1, 14}, false, "NcString"},
			{WorkerTypeTX, "Transmitter", "TX_ALL", []int{1, 2, 1, 15}, true, "NcString"},
			{WorkerTypeBattery, "Battery", "BATT_ALL", []int{1, 2, 1, 16}, true, "NcString"},
			{WorkerTypeAudioLevel, "AudioLevel", "AUDIO_LEVEL", []int{1, 2, 1, 17}, true, "NcString"},
			{WorkerTypeRSSI, "RSSI", "RF_RSSI", []int{1, 2, 1, 18}, true, "NcString"},
		}
	case ModelFamilyULXD:
		return []ChannelWorkerConfig{
			{WorkerTypeGain, "Gain", "AUDIO_GAIN", []int{1, 2, 1, 10}, false, "NcFloat32"},
			{WorkerTypeMute, "Mute", "AUDIO_MUTE", []int{1, 2, 1, 11}, false, "NcBoolean"},
			{WorkerTypeChannelName, "ChannelName", "CHAN_NAME", []int{1, 2, 1, 12}, false, "NcString"},
			{WorkerTypeFreq, "Frequency", "FREQUENCY", []int{1, 2, 1, 13}, false, "NcString"},
			{WorkerTypeGroupChan, "GroupChannel", "GROUP_CHAN", []int{1, 2, 1, 14}, false, "NcString"},
			{WorkerTypeTX, "Transmitter", "TX_TYPE", []int{1, 2, 1, 15}, true, "NcString"},
			{WorkerTypeBattery, "Battery", "TX_BATT_MINS", []int{1, 2, 1, 16}, true, "NcString"},
			{WorkerTypeAudioLevel, "AudioLevel", "AUDIO_LVL", []int{1, 2, 1, 17}, true, "NcString"},
			{WorkerTypeRSSI, "RSSI", "RX_RF_LVL", []int{1, 2, 1, 18}, true, "NcString"},
		}
	case ModelFamilyQLXD:
		return []ChannelWorkerConfig{
			{WorkerTypeChannelName, "ChannelName", "CHAN_NAME", []int{1, 2, 1, 12}, false, "NcString"},
			{WorkerTypeFreq, "Frequency", "FREQUENCY", []int{1, 2, 1, 13}, false, "NcString"},
			{WorkerTypeGroupChan, "GroupChannel", "GROUP_CHAN", []int{1, 2, 1, 14}, false, "NcString"},
			{WorkerTypeEncryptionWarning, "EncryptionWarning", "ENCRYPTION_WARNING", []int{1, 2, 1, 40}, true, "NcBoolean"},
			{WorkerTypeAntStatus, "AntennaStatus", "ANT_STATUS", []int{1, 2, 1, 41}, true, "NcString"},
			{WorkerTypeRFLevel, "RFLevel", "RF_LEVEL", []int{1, 2, 1, 42}, true, "NcString"},
			{WorkerTypeAudioLevelMeter, "AudioLevel", "AUDIO_LEVEL", []int{1, 2, 1, 43}, true, "NcString"},
		}
	case ModelFamilySLXD:
		return []ChannelWorkerConfig{
			{WorkerTypeGain, "Gain", "AUDIO_GAIN", []int{1, 2, 1, 10}, false, "NcFloat32"},
			{WorkerTypeMute, "Mute", "AUDIO_MUTE", []int{1, 2, 1, 11}, false, "NcBoolean"},
			{WorkerTypeChannelName, "ChannelName", "CHAN_NAME", []int{1, 2, 1, 12}, false, "NcString"},
			{WorkerTypeFreq, "Frequency", "FREQUENCY", []int{1, 2, 1, 13}, false, "NcString"},
			{WorkerTypeGroupChan, "GroupChannel", "GROUP_CHANNEL", []int{1, 2, 1, 14}, false, "NcString"},
			{WorkerTypeTX, "Transmitter", "TX_MODEL", []int{1, 2, 1, 15}, true, "NcString"},
			{WorkerTypeBattery, "Battery", "TX_BATT_MINS", []int{1, 2, 1, 16}, true, "NcString"},
			{WorkerTypeAudioLevel, "AudioLevel", "AUDIO_LEVEL_RMS", []int{1, 2, 1, 17}, true, "NcString"},
			{WorkerTypeRSSI, "RSSI", "RSSI", []int{1, 2, 1, 18}, true, "NcString"},
		}
	case ModelFamilySLXDPlus:
		return []ChannelWorkerConfig{
			{WorkerTypeGain, "Gain", "AUDIO_GAIN", []int{1, 2, 1, 10}, false, "NcFloat32"},
			{WorkerTypeMute, "Mute", "AUDIO_MUTE", []int{1, 2, 1, 11}, false, "NcBoolean"},
			{WorkerTypeChannelName, "ChannelName", "CHAN_NAME", []int{1, 2, 1, 12}, false, "NcString"},
			{WorkerTypeFreq, "Frequency", "FREQUENCY", []int{1, 2, 1, 13}, false, "NcString"},
			{WorkerTypeGroupChan, "GroupChannel", "GROUP_CHANNEL", []int{1, 2, 1, 14}, false, "NcString"},
			{WorkerTypeTX, "Transmitter", "LINK_TX_MODEL", []int{1, 2, 1, 15}, true, "NcString"},
			{WorkerTypeBattery, "Battery", "TX_BATT_MINS", []int{1, 2, 1, 16}, true, "NcString"},
			{WorkerTypeAudioLevel, "AudioLevel", "AUDIO_LEVEL_RMS", []int{1, 2, 1, 17}, true, "NcString"},
			{WorkerTypeRSSI, "RSSI", "RSSI", []int{1, 2, 1, 18}, true, "NcString"},
		}
	default:
		return nil
	}
}

// CreateChannelWorkers creates all workers for a channel based on model family
// Returns workers and a map of channel_paramKey -> OID for REP message routing
func CreateChannelWorkers(family ShureModelFamily, channel int, baseOID int, ctrl ShureController) ([]*NcWorker, map[string]int) {
	configs := ChannelWorkersConfig(family)
	if configs == nil {
		return nil, nil
	}

	workers := make([]*NcWorker, 0, len(configs))
	paramOIDMap := make(map[string]int)

	for i, cfg := range configs {
		oid := baseOID + i
		worker := NewNcWorker(oid, cfg.ClassID, nil, cfg.Name, fmt.Sprintf("Ch%d %s", channel, cfg.Name))

		if !cfg.IsReadOnly {
			paramName := cfg.ShureParam
			channelNum := channel
			worker.OnSet = func(val interface{}) error {
				cmd := fmt.Sprintf("< SET %d %s %v >\n", channelNum, paramName, val)
				return ctrl.SendCommand(cmd)
			}
		}

		paramKey := fmt.Sprintf("%d_%s", channel, cfg.ShureParam)
		paramOIDMap[paramKey] = oid

		workers = append(workers, worker)
	}

	return workers, paramOIDMap
}

// DeviceWorkerConfig defines a single parameter worker for the device (not channel-specific)
type DeviceWorkerConfig struct {
	WorkerType  WorkerType
	Name        string
	ShureParam  string
	ClassID     []int
	IsReadOnly  bool
	TypeName    string
	NeedsChannel bool // Some device params still need channel number (e.g., CHAN_NAME for QLX-D)
}

// DeviceWorkersConfig returns the list of device-level workers for a model family
func DeviceWorkersConfig(family ShureModelFamily) []DeviceWorkerConfig {
	switch family {
	case ModelFamilyQLXD:
		return []DeviceWorkerConfig{
			{WorkerTypeFWVer, "FwVersion", "FW_VER", []int{1, 2, 1, 19}, true, "NcString", false},
			{WorkerTypeDeviceID, "DeviceID", "DEVICE_ID", []int{1, 2, 1, 20}, false, "NcString", false},
			{WorkerTypeEncryption, "Encryption", "ENCRYPTION", []int{1, 2, 1, 21}, false, "NcBoolean", false},
			{WorkerTypeMACAddr, "MacAddress", "MAC_ADDR", []int{1, 2, 1, 22}, true, "NcString", false},
		}
	default:
		return nil
	}
}

// CreateDeviceWorkers creates all device-level workers for QLX-D
func CreateDeviceWorkers(family ShureModelFamily, baseOID int, ctrl ShureController) ([]*NcWorker, map[string]int) {
	configs := DeviceWorkersConfig(family)
	if configs == nil {
		return nil, nil
	}

	workers := make([]*NcWorker, 0, len(configs))
	paramOIDMap := make(map[string]int)

	for i, cfg := range configs {
		oid := baseOID + i
		worker := NewNcWorker(oid, cfg.ClassID, nil, cfg.Name, cfg.Name)

		if !cfg.IsReadOnly {
			paramName := cfg.ShureParam
			worker.OnSet = func(val interface{}) error {
				cmd := fmt.Sprintf("< SET %s %v >\n", paramName, val)
				return ctrl.SendCommand(cmd)
			}
		}

		paramOIDMap[cfg.ShureParam] = oid
		workers = append(workers, worker)
	}

	return workers, paramOIDMap
}

// SubWorkerConfig defines a worker within a nested block (Battery, TX, etc.)
type SubWorkerConfig struct {
	WorkerType WorkerType
	Name       string
	ShureParam string
	ClassID    []int
	IsReadOnly bool
	TypeName   string
}

// ChannelSubWorkersConfig returns the nested block workers for a channel
func ChannelSubWorkersConfig(family ShureModelFamily) map[string][]SubWorkerConfig {
	switch family {
	case ModelFamilyQLXD:
		return map[string][]SubWorkerConfig{
			"AudioGain": {
				{WorkerTypeGain, "SetAudioGain", "AUDIO_GAIN", []int{1, 2, 1, 10}, false, "NcFloat32"},
			},
			"Battery": {
				{WorkerTypeBattCycle, "BattCycle", "BATT_CYCLE", []int{1, 2, 1, 23}, true, "NcString"},
				{WorkerTypeBattRunTime, "BattRunTime", "BATT_RUN_TIME", []int{1, 2, 1, 24}, true, "NcString"},
				{WorkerTypeBattTempF, "BattTempF", "BATT_TEMP_F", []int{1, 2, 1, 25}, true, "NcString"},
				{WorkerTypeBattTempC, "BattTempC", "BATT_TEMP_C", []int{1, 2, 1, 26}, true, "NcString"},
				{WorkerTypeBattType, "BattType", "BATT_TYPE", []int{1, 2, 1, 27}, true, "NcString"},
				{WorkerTypeBattCharge, "BattCharge", "BATT_CHARGE", []int{1, 2, 1, 28}, true, "NcString"},
				{WorkerTypeBattHealth, "BattHealth", "BATT_HEALTH", []int{1, 2, 1, 29}, true, "NcString"},
				{WorkerTypeBattBars, "BattBars", "BATT_BARS", []int{1, 2, 1, 30}, true, "NcString"},
			},
			"Transmitter": {
				{WorkerTypeTxType, "TxType", "TX_TYPE", []int{1, 2, 1, 31}, true, "NcString"},
				{WorkerTypeTxOffset, "TxOffset", "TX_OFFSET", []int{1, 2, 1, 32}, true, "NcString"},
				{WorkerTypeTxRFPower, "TxRFPower", "TX_RF_PWR", []int{1, 2, 1, 33}, true, "NcString"},
				{WorkerTypeTxPwrLock, "TxPwrLock", "TX_PWR_LOCK", []int{1, 2, 1, 34}, true, "NcString"},
				{WorkerTypeTxMenuLock, "TxMenuLock", "TX_MENU_LOCK", []int{1, 2, 1, 35}, true, "NcString"},
				{WorkerTypeTxDeviceID, "TxDeviceID", "TX_DEVICE_ID", []int{1, 2, 1, 36}, true, "NcString"},
				{WorkerTypeTxMuteStatus, "TxMuteStatus", "TX_MUTE_STATUS", []int{1, 2, 1, 37}, true, "NcString"},
				{WorkerTypeTxMuteButtonStatus, "TxMuteButtonStatus", "TX_MUTE_BUTTON_STATUS", []int{1, 2, 1, 38}, true, "NcString"},
				{WorkerTypeTxPowerSource, "TxPowerSource", "TX_POWER_SOURCE", []int{1, 2, 1, 39}, true, "NcString"},
			},
		}
	default:
		return nil
	}
}

// CreateChannelSubWorkers creates sub-block workers for a channel
func CreateChannelSubWorkers(family ShureModelFamily, channel int, baseOID int, ctrl ShureController) (map[string][]*NcWorker, map[string]map[string]int) {
	subConfigs := ChannelSubWorkersConfig(family)
	if subConfigs == nil {
		return nil, nil
	}

	resultWorkers := make(map[string][]*NcWorker)
	resultParamMap := make(map[string]map[string]int)

	for blockName, configs := range subConfigs {
		workers := make([]*NcWorker, 0, len(configs))
		paramMap := make(map[string]int)

		for i, cfg := range configs {
			oid := baseOID + i
			worker := NewNcWorker(oid, cfg.ClassID, nil, cfg.Name, fmt.Sprintf("Ch%d %s %s", channel, blockName, cfg.Name))

			if !cfg.IsReadOnly {
				paramName := cfg.ShureParam
				channelNum := channel
				worker.OnSet = func(val interface{}) error {
					cmd := fmt.Sprintf("< SET %d %s %v >\n", channelNum, paramName, val)
					return ctrl.SendCommand(cmd)
				}
			}

			paramMap[cfg.ShureParam] = oid

			workers = append(workers, worker)
		}

		resultWorkers[blockName] = workers
		resultParamMap[blockName] = paramMap
	}

	return resultWorkers, resultParamMap
}