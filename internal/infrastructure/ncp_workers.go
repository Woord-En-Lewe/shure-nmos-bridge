package infrastructure

import (
	"fmt"
)

// ParameterDefinition defines a single parameter for a model family
type ParameterDefinition struct {
	Name        string
	ClassID     []int
	IsReadOnly  bool
	TypeName    string
	Description string
}

// ModelFamilyParameters maps model family to its channel parameter definitions
// Based on official Shure command string documentation for each model
var ModelFamilyParameters = map[ShureModelFamily][]ParameterDefinition{
	ModelFamilyAxientDigital: {
		// Channel Control Parameters
		{Name: "AUDIO_GAIN", ClassID: []int{1, 2, 1, 1}, IsReadOnly: false, TypeName: "NcFloat32", Description: "Channel audio gain in dB (range -18 to 42)"},
		{Name: "AUDIO_MUTE", ClassID: []int{1, 2, 1, 2}, IsReadOnly: false, TypeName: "NcBoolean", Description: "Channel audio mute state"},
		{Name: "CHAN_NAME", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Channel name (31 chars max)"},
		{Name: "FREQUENCY", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Frequency in MHz"},
		{Name: "GROUP_CHANNEL", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Group/Channel mapping (format: gg,cc)"},
		{Name: "FD_MODE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Frequency diversity mode (OFF, FD-C, FD-S)"},
		{Name: "ENCRYPTION_STATUS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Encryption status (OK, ERROR)"},
		{Name: "ENCRYPTION_MODE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Encryption mode (ON, OFF)"},
		{Name: "INTERFERENCE_STATUS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Interference detection status (NONE, DETECTED)"},
		{Name: "UNREGISTERED_TX_STATUS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Unregistered transmitter status (OK, ERROR)"},
		{Name: "QUADVERSITY_MODE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Quadversity mode (ON, OFF)"},
		// RF Status (read-only)
		{Name: "RF_BAND", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "RF band"},
		{Name: "TRANSMISSION_MODE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmission mode (STANDARD, HIGH_DENSITY)"},
		// Transmitter Battery Parameters (read-only)
		{Name: "TX_BATT_BARS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Transmitter battery bars (0-5, 255=unknown)"},
		{Name: "TX_BATT_CHARGE_PERCENT", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Transmitter battery charge % (0-100, 255=unknown)"},
		{Name: "TX_BATT_CYCLE_COUNT", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Transmitter battery cycle count"},
		{Name: "TX_BATT_HEALTH_PERCENT", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Transmitter battery health %"},
		{Name: "TX_BATT_MINS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Transmitter battery runtime minutes"},
		{Name: "TX_BATT_TEMP_C", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcInt32", Description: "Transmitter battery temperature °C"},
		{Name: "TX_BATT_TYPE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter battery type (LION, ALKA, NIMH, LITH, UNKN)"},
		{Name: "TX_DEVICE_ID", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter device ID"},
		{Name: "TX_INPUT_PAD", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcInt32", Description: "Transmitter input pad status"},
		{Name: "TX_LOCK", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter lock status (NONE, POWER, MENU, ALL, UNKNOWN)"},
		{Name: "TX_MODEL", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter model (AD1, AD2, ADX1, ADX1M, ADX2, ADX2FD, UNKNOWN)"},
		{Name: "TX_MUTE_MODE_STATUS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter mute mode status (ON, MUTE, UNKNOWN)"},
		{Name: "TX_OFFSET", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcInt32", Description: "Transmitter offset (dB, range -12 to +21)"},
		{Name: "TX_POLARITY", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter polarity (POSITIVE, NEGATIVE, UNKNOWN)"},
		{Name: "TX_POWER_LEVEL", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Transmitter RF power level (mW)"},
		{Name: "TX_TALK_SWITCH", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter talk switch status (ON, OFF, UNKNOWN)"},
		// Metering Parameters (read-only)
		{Name: "CHAN_QUALITY", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Channel quality (0-5)"},
		{Name: "AUDIO_LEVEL_PEAK", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcFloat32", Description: "Audio peak level (dBFS)"},
		{Name: "AUDIO_LEVEL_RMS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcFloat32", Description: "Audio RMS level (dBFS)"},
		{Name: "AUDIO_LED_BITMAP", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Audio LED bitmap"},
		{Name: "ANTENNA_STATUS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Antenna status (X=Off, R=Red, B=Blue)"},
		{Name: "RSSI", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcFloat32", Description: "RF RSSI (dBm)"},
		{Name: "RSSI_LED_BITMAP", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "RSSI LED bitmap"},
	},
	ModelFamilyULXD: {
		// Channel Control Parameters
		{Name: "AUDIO_GAIN", ClassID: []int{1, 2, 1, 1}, IsReadOnly: false, TypeName: "NcFloat32", Description: "Channel audio gain in dB (range -18 to 42)"},
		{Name: "AUDIO_MUTE", ClassID: []int{1, 2, 1, 2}, IsReadOnly: false, TypeName: "NcBoolean", Description: "Channel audio mute state"},
		{Name: "CHAN_NAME", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Channel name (8 chars max)"},
		{Name: "FREQUENCY", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Frequency in KHz"},
		{Name: "GROUP_CHAN", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Group/Channel mapping (format: gg,cc)"},
		{Name: "ENCRYPTION_WARNING", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Encryption mismatch status (OFF, ON)"},
		{Name: "RF_INT_DET", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "RF interference detection (NONE, CRITICAL)"},
		// Metering Parameters (read-only)
		{Name: "RF_ANTENNA", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "RF antenna status (AX, XB, AB, XX)"},
		{Name: "RX_RF_LVL", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcFloat32", Description: "RF received level (dBm)"},
		{Name: "AUDIO_LVL", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcFloat32", Description: "Audio level (dBFS)"},
		// Transmitter Battery Parameters (read-only)
		{Name: "BATT_BARS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Battery bars (0-5, 255=unknown)"},
		{Name: "BATT_CHARGE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Battery charge % (0-100, 255=unknown)"},
		{Name: "BATT_CYCLE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Battery cycle count"},
		{Name: "BATT_HEALTH", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Battery health %"},
		{Name: "BATT_RUN_TIME", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Battery runtime minutes"},
		{Name: "BATT_TEMP_C", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcInt32", Description: "Battery temperature °C"},
		{Name: "BATT_TYPE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Battery type (LION, ALKA, NIMH, LITH, WARN, UNKN)"},
		{Name: "TX_DEVICE_ID", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter device ID"},
		{Name: "TX_FW_VER", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter firmware version"},
		{Name: "TX_MENU_LOCK", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter menu lock status (ON, OFF, UNKN)"},
		{Name: "TX_MUTE_BUTTON_STATUS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter mute button status (PRESSED, RELEASED, UNKN)"},
		{Name: "TX_MUTE_STATUS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter mute status (ON, OFF, UNKN)"},
		{Name: "TX_OFFSET", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcInt32", Description: "Transmitter offset (dB, range -12 to +21)"},
		{Name: "TX_POWER_SOURCE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter power source (BATTERY, EXTERNAL, UNKNOWN)"},
		{Name: "TX_PWR_LOCK", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter power lock status (ON, OFF, UNKN)"},
		{Name: "TX_RF_PWR", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter RF power (LOW, NORMAL, HIGH, UNKN)"},
		{Name: "TX_TYPE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter type (QLXD1, QLXD2, ULXD1, ULXD2, ULXD6, ULXD8, UNKN)"},
	},
	ModelFamilyQLXD: {
		// Channel Control Parameters
		{Name: "AUDIO_GAIN", ClassID: []int{1, 2, 1, 1}, IsReadOnly: false, TypeName: "NcFloat32", Description: "Channel audio gain in dB (range -18 to 42)"},
		{Name: "AUDIO_MUTE", ClassID: []int{1, 2, 1, 2}, IsReadOnly: false, TypeName: "NcBoolean", Description: "Channel audio mute state"},
		{Name: "CHAN_NAME", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Channel name (8 chars max)"},
		{Name: "FREQUENCY", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Frequency in MHz (format: yyy.yyy)"},
		{Name: "GROUP_CHAN", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Group/Channel mapping (format: gg,cc)"},
		// Transmitter Battery Parameters (read-only)
		{Name: "BATT_BARS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Battery bars (0-5, 255=unknown)"},
		{Name: "BATT_CHARGE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Battery charge % (0-100)"},
		{Name: "BATT_HEALTH", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Battery health %"},
		{Name: "BATT_RUN_TIME", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Battery runtime minutes"},
		{Name: "BATT_TEMP_C", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcInt32", Description: "Battery temperature °C (offset by 40)"},
		{Name: "BATT_TEMP_F", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcInt32", Description: "Battery temperature °F (offset by 40)"},
		{Name: "BATT_CYCLE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Battery cycle count"},
		{Name: "BATT_TYPE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Battery type (LION, ALKA, NIMH, LITH, UNKN)"},
		// Transmitter Parameters (read-only)
		{Name: "TX_TYPE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter type (QLXD1, QLXD2, ULXD1, ULXD2, ULXD6, ULXD8, UNKN)"},
		{Name: "TX_OFFSET", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Transmitter offset (dB)"},
		{Name: "TX_RF_PWR", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter RF power (LOW, HIGH, UNKN)"},
		{Name: "TX_PWR_LOCK", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter power lock status (ON, OFF, UNKN)"},
		{Name: "TX_MENU_LOCK", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter menu lock status (ON, OFF, UNKN)"},
		{Name: "TX_DEVICE_ID", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter device ID"},
		{Name: "TX_MUTE_STATUS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter mute status (ON, OFF, UNKN)"},
		{Name: "TX_MUTE_BUTTON_STATUS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter mute button status (PRESSED, RELEASED, UNKN)"},
		{Name: "TX_POWER_SOURCE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter power source (BATTERY, EXTERNAL, UNKN)"},
		{Name: "ENCRYPTION_WARNING", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Encryption mismatch status (OFF, ON)"},
		// Metering Parameters (read-only)
		{Name: "RF_ANTENNA", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "RF antenna status (AX, XB, AB, XX)"},
	},
	ModelFamilySLXD: {
		// Channel Control Parameters
		{Name: "AUDIO_GAIN", ClassID: []int{1, 2, 1, 1}, IsReadOnly: false, TypeName: "NcFloat32", Description: "Channel audio gain in dB (range -18 to 42)"},
		{Name: "AUDIO_MUTE", ClassID: []int{1, 2, 1, 2}, IsReadOnly: false, TypeName: "NcBoolean", Description: "Channel audio mute state"},
		{Name: "CHAN_NAME", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Channel name (31 chars max, SET takes 8)"},
		{Name: "FREQUENCY", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Frequency in KHz"},
		{Name: "GROUP_CHANNEL", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Group/Channel mapping"},
		// Metering Parameters (read-only)
		{Name: "AUDIO_LEVEL_PEAK", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcFloat32", Description: "Audio peak level (dBFS)"},
		{Name: "AUDIO_LEVEL_RMS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcFloat32", Description: "Audio RMS level (dBFS)"},
		{Name: "RSSI", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcFloat32", Description: "RF RSSI (dBm)"},
		// Transmitter Parameters (read-only)
		{Name: "TX_MODEL", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Transmitter model (SLXD1, SLXD2, UNKNOWN)"},
		{Name: "TX_BATT_BARS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Transmitter battery bars (0-5, 255=unknown)"},
		{Name: "TX_BATT_MINS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Transmitter battery runtime minutes"},
	},
	ModelFamilySLXDPlus: {
		// Channel Control Parameters
		{Name: "AUDIO_GAIN", ClassID: []int{1, 2, 1, 1}, IsReadOnly: false, TypeName: "NcFloat32", Description: "Channel audio gain in dB (range -18 to 42)"},
		{Name: "AUDIO_MUTE", ClassID: []int{1, 2, 1, 2}, IsReadOnly: false, TypeName: "NcBoolean", Description: "Channel audio mute state"},
		{Name: "CHAN_NAME", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Channel name (31 chars max)"},
		{Name: "AUDIO_OUT_LVL_SWITCH", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Audio output level switch (MIC, LINE)"},
		{Name: "GROUP_CHANNEL", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Group/Channel mapping"},
		{Name: "REM_PAIR", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Remote pairing status (ON, OFF, REQUEST, ACCEPT, REJECT)"},
		{Name: "INTERFERENCE_STATUS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Interference status (NONE, DETECTED)"},
		{Name: "ENCRYPTION_STATUS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Encryption status (OK, ERROR)"},
		{Name: "ENCRYPTION_MODE", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Encryption mode (ON, OFF)"},
		{Name: "FREQUENCY", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Frequency in KHz"},
		// Metering Parameters (read-only)
		{Name: "AUDIO_LEVEL_PEAK", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcFloat32", Description: "Audio peak level (dBFS)"},
		{Name: "AUDIO_LEVEL_RMS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcFloat32", Description: "Audio RMS level (dBFS)"},
		{Name: "RSSI", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcFloat32", Description: "RF RSSI (dBm)"},
		// Transmitter Parameters (read-only)
		{Name: "LINK_TX_MODEL", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Linked transmitter model (SLXD1+, SLXD2+, SLXD3+, UNKNOWN)"},
		{Name: "LINK_STATUS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcString", Description: "Link status (EMPTY, LINKED.INACTIVE, LINKED.ACTIVE)"},
		{Name: "LINK_TX_BATT_MINS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Linked transmitter battery runtime minutes"},
		{Name: "TX_BATT_BARS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Transmitter battery bars"},
		{Name: "TX_BATT_MINS", ClassID: []int{1, 2, 2}, IsReadOnly: true, TypeName: "NcUint32", Description: "Transmitter battery runtime minutes"},
		// Transmitter Control Parameters
		{Name: "LINK_TX_REBOOT", ClassID: []int{1, 2, 2}, IsReadOnly: false, TypeName: "NcString", Description: "Reboot linked transmitter command"},
	},
}

// GetParametersForFamily returns parameter definitions for a model family
func GetParametersForFamily(family ShureModelFamily) []ParameterDefinition {
	if params, ok := ModelFamilyParameters[family]; ok {
		return params
	}
	return nil
}

// CreateWorkersForChannel creates all workers for a specific channel
func CreateWorkersForChannel(family ShureModelFamily, channel int, baseOID int, ctrl ShureController) []*NcWorker {
	params := GetParametersForFamily(family)
	if params == nil {
		return nil
	}

	workers := make([]*NcWorker, 0, len(params))

	for i, param := range params {
		oid := baseOID + i
		worker := NewNcWorker(oid, param.ClassID, nil, param.Name, fmt.Sprintf("Channel %d %s", channel, param.Name))

		if !param.IsReadOnly {
			paramName := param.Name
			channelNum := channel
			worker.OnSet = func(val interface{}) error {
				cmd := fmt.Sprintf("< SET %d %s %v >\n", channelNum, paramName, val)
				return ctrl.SendCommand(cmd)
			}
		}

		workers = append(workers, worker)
	}

	return workers
}