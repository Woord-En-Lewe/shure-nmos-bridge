package infrastructure

import (
	"fmt"
	"strconv"
	"strings"
)

// ShureCommandBuilder implements the builder pattern for creating Shure TPCI commands.
// Axient Digital Format: < COMMAND [INDEX] PARAMETER {VALUE} >
type ShureCommandBuilder struct {
	command string
	index   int
	params  map[string]interface{}
}

// NewShureCommand creates a new command builder
func NewShureCommand(command string) *ShureCommandBuilder {
	return &ShureCommandBuilder{
		command: strings.ToUpper(command),
		index:   0,
		params:  make(map[string]interface{}),
	}
}

func (b *ShureCommandBuilder) WithIndex(index int) *ShureCommandBuilder {
	b.index = index
	return b
}

func (b *ShureCommandBuilder) WithParam(key string, value interface{}) *ShureCommandBuilder {
	b.params[strings.ToUpper(key)] = value
	return b
}

func (b *ShureCommandBuilder) Build() string {
	var sb strings.Builder
	sb.WriteString("< ")
	sb.WriteString(b.command)
	sb.WriteString(" ")

	// Device level is 0, channels are 1-4
	if b.index > 0 || (b.command == "GET" && b.index == 0 && len(b.params) > 0) {
		// Note: Axient allows < GET 0 ALL >
		sb.WriteString(strconv.Itoa(b.index))
		sb.WriteString(" ")
	}

	for key, value := range b.params {
		sb.WriteString(key)
		if value != nil {
			sb.WriteString(" ")
			var valueStr string
			switch v := value.(type) {
			case string:
				// Axient names are wrapped in braces
				valueStr = fmt.Sprintf("{%s}", v)
			case int:
				valueStr = strconv.Itoa(v)
			case bool:
				if v {
					valueStr = "ON"
				} else {
					valueStr = "OFF"
				}
			case HighDensityMode:
				if v {
					valueStr = "ON"
				} else {
					valueStr = "OFF"
				}
			case EncryptionWarning:
				if v {
					valueStr = "ON"
				} else {
					valueStr = "OFF"
				}
			case AxientGain:
				valueStr = fmt.Sprintf("%03d", int(v))
			case ULXDGain:
				valueStr = fmt.Sprintf("%03d", int(v))
			case SLDXAudioLevel:
				valueStr = fmt.Sprintf("%03d", int(v))
			case AudioOutputLevelSwitch:
				valueStr = string(v)
			case AudioSummingMode:
				valueStr = string(v)
			case FrequencyDiversityMode:
				valueStr = string(v)
			case QuadversityMode:
				valueStr = string(v)
			case FDMode:
				valueStr = string(v)
			case EncryptionMode:
				valueStr = string(v)
			case RemotePairStatus:
				valueStr = string(v)
			case RemotePairAction:
				valueStr = string(v)
			case LinkTXStatus:
				valueStr = string(v)
			case EncryptionStatus:
				valueStr = string(v)
			case AntennaSquelchStatus:
				valueStr = string(v)
			case TXMuteStatus:
				valueStr = string(v)
			case TXMuteButtonStatus:
				valueStr = string(v)
			case TXPowerSource:
				valueStr = string(v)
			case BatteryType:
				valueStr = string(v)
			case TXLock:
				valueStr = string(v)
			case SlotStatus:
				valueStr = string(v)
			case SlotProperty:
				valueStr = string(v)
			case InterferenceStatus:
				valueStr = string(v)
			case NetworkInterface:
				valueStr = string(v)
			case IPMode:
				valueStr = string(v)
			default:
				valueStr = fmt.Sprintf("%v", v)
			}
			sb.WriteString(valueStr)
		}
		sb.WriteString(" ")
	}

	sb.WriteString(">")
	return sb.String()
}

// Domain-specific types for Axient Digital

// ShureModelFamily represents the different Shure receiver families
type ShureModelFamily string

const (
	ModelFamilyAxientDigital ShureModelFamily = "axient_digital"
	ModelFamilyULXD          ShureModelFamily = "ulx_d"
	ModelFamilyQLXD          ShureModelFamily = "qlx_d"
	ModelFamilySLXD          ShureModelFamily = "slx_d"
	ModelFamilySLXDPlus      ShureModelFamily = "slx_d_plus"
)

// UseSpaces returns true if this model family uses spaces in parameter names
// instead of underscores (ULX-D and QLX-D use spaces)
func (m ShureModelFamily) UseSpaces() bool {
	return m == ModelFamilyULXD || m == ModelFamilyQLXD
}

// FormatParamName formats a parameter name according to the model family's conventions
func (m ShureModelFamily) FormatParamName(param string) string {
	if m.UseSpaces() {
		return strings.ReplaceAll(strings.ToUpper(param), "_", " ")
	}
	return strings.ToUpper(param)
}

// DetectModelFamily attempts to detect the model family from a MODEL string
func DetectModelFamily(model string) ShureModelFamily {
	modelUpper := strings.ToUpper(model)
	if strings.HasPrefix(modelUpper, "AD") {
		return ModelFamilyAxientDigital
	}
	if strings.HasPrefix(modelUpper, "ULXD") {
		return ModelFamilyULXD
	}
	if strings.HasPrefix(modelUpper, "QLXD") {
		return ModelFamilyQLXD
	}
	if strings.Contains(modelUpper, "+") || strings.Contains(modelUpper, "SLXD+") {
		return ModelFamilySLXDPlus
	}
	if strings.HasPrefix(modelUpper, "SLXD") {
		return ModelFamilySLXD
	}
	return ModelFamilyAxientDigital
}

// ShureCommandBuilderWithModel creates a command builder with model family awareness
type ShureCommandBuilderWithModel struct {
	command string
	index   int
	params  map[string]interface{}
	family  ShureModelFamily
}

// NewShureCommandWithModel creates a new command builder for a specific model family
func NewShureCommandWithModel(command string, family ShureModelFamily) *ShureCommandBuilderWithModel {
	return &ShureCommandBuilderWithModel{
		command: strings.ToUpper(command),
		index:   0,
		params:  make(map[string]interface{}),
		family:  family,
	}
}

func (b *ShureCommandBuilderWithModel) WithIndex(index int) *ShureCommandBuilderWithModel {
	b.index = index
	return b
}

func (b *ShureCommandBuilderWithModel) WithParam(key string, value interface{}) *ShureCommandBuilderWithModel {
	b.params[strings.ToUpper(key)] = value
	return b
}

func (b *ShureCommandBuilderWithModel) Build() string {
	var sb strings.Builder
	sb.WriteString("< ")
	sb.WriteString(b.command)
	sb.WriteString(" ")

	if b.index > 0 || (b.command == "GET" && b.index == 0 && len(b.params) > 0) {
		sb.WriteString(strconv.Itoa(b.index))
		sb.WriteString(" ")
	}

	for key, value := range b.params {
		formattedKey := b.family.FormatParamName(key)
		sb.WriteString(formattedKey)
		if value != nil {
			sb.WriteString(" ")
			var valueStr string
			switch v := value.(type) {
			case string:
				if b.family == ModelFamilyAxientDigital || b.family == ModelFamilySLXDPlus || b.family == ModelFamilySLXD {
					valueStr = fmt.Sprintf("{%s}", v)
				} else {
					valueStr = v
				}
			case int:
				valueStr = strconv.Itoa(v)
			case bool:
				if v {
					valueStr = "ON"
				} else {
					valueStr = "OFF"
				}
			case AxientGain:
				valueStr = fmt.Sprintf("%03d", int(v))
			case ULXDGain:
				valueStr = fmt.Sprintf("%03d", int(v))
			case SLDXAudioLevel:
				valueStr = fmt.Sprintf("%03d", int(v))
			default:
				valueStr = fmt.Sprintf("%v", v)
			}
			sb.WriteString(valueStr)
		}
		sb.WriteString(" ")
	}

	sb.WriteString(">")
	return sb.String()
}

// AxientGain handles the 18dB offset (TPCI 030 = 12dB)
type AxientGain int

func (g AxientGain) ToDB() int      { return int(g) - 18 }
func (g AxientGain) String() string { return fmt.Sprintf("%03d", int(g)) }

// ULXDGain handles the 18dB offset for ULX-D/QLX-D (TPCI 030 = 12dB)
type ULXDGain int

func (g ULXDGain) ToDB() int      { return int(g) - 18 }
func (g ULXDGain) String() string { return fmt.Sprintf("%03d", int(g)) }

// SLDXAudioLevel handles SLX-D/SLX-D+ audio levels (000-120, subtract 120 for dBFS)
type SLDXAudioLevel int

func (l SLDXAudioLevel) ToDBFS() int { return int(l) - 120 }

// ULXDRFLevel handles ULX-D/QLX-D RF levels (000-115, subtract 128 for dBm)
type ULXDRFLevel int

func (l ULXDRFLevel) ToDBM() int { return int(l) - 128 }

// AxientLevel handles the 120dBFS offset (TPCI 102 = -18dBFS)
type AxientLevel int

func (l AxientLevel) ToDBFS() int { return int(l) - 120 }

// Gain represents audio gain in dB
type Gain float64

func NewGain(v float64) Gain { return Gain(v) }

func (g Gain) String() string { return fmt.Sprintf("%v", float64(g)) }

// Mute represents mute state (0=off, 1=on)
type Mute bool

func NewMute(v bool) Mute { return Mute(v) }

func (m Mute) String() string {
	if m {
		return "1"
	}
	return "0"
}

// Frequency represents frequency in MHz
type Frequency float64

func NewFrequency(v float64) Frequency { return Frequency(v) }

func (f Frequency) String() string { return fmt.Sprintf("%v", float64(f)) }

// Channel represents channel number
type Channel int

func NewChannel(v int) Channel { return Channel(v) }

func (c Channel) String() string { return fmt.Sprintf("%d", int(c)) }

// DeviceID represents a Shure device ID
type DeviceID string

func NewDeviceID(v string) DeviceID { return DeviceID(v) }

func (d DeviceID) String() string { return string(d) }

// TPCIReport represents a parsed Axient Digital response
type TPCIReport struct {
	Type    string // REP or SAMPLE
	Channel int
	Param   string
	Value   string
	Raw     string
}

// IsMeteredParam returns true if the parameter is a metered property
func IsMeteredParam(param string) bool {
	switch param {
	case "CHAN_QUALITY", "AUDIO_LED_BITMAP", "AUDIO_LEVEL_PEAK", "AUDIO_LEVEL_RMS",
		"ANTENNA_STATUS", "RF_LED_BITMAP_A", "RF_RSSI_A", "RF_LED_BITMAP_B", "RF_RSSI_B",
		"RF_LED_BITMAP_C", "RF_RSSI_C", "RF_LED_BITMAP_D", "RF_RSSI_D",
		"RF_LED_BITMAP_F1", "RF_RSSI_F1", "RF_LED_BITMAP_F2", "RF_RSSI_F2",
		"AUDIO_SUMMING", "RF_LEVEL", "AUDIO_PEAK", "AUDIO_RMS", "RF_RSSI",
		"TX_BATT_BARS", "TX_BATT_CHARGE_PERCENT", "TX_BATT_MINS", "TX_BATT_TEMP_C",
		"TX_BATT_CYCLE_COUNT", "TX_BATT_HEALTH_PERCENT":
		return true
	default:
		return false
	}
}

// ParseTPCIResponse handles the Axient TPCI format including padded strings
func ParseTPCIResponse(response string) *TPCIReport {
	// Example 1: < REP 1 CHAN_NAME {Lead Vox       } >
	// Example 2: < SAMPLE 1 ALL 005 000 045 062 BB 31 099 31 085 >
	trimmed := strings.Trim(response, "<> ")
	parts := strings.Fields(trimmed)

	if len(parts) < 2 {
		return nil
	}

	msgType := parts[0]
	if msgType != "REP" && msgType != "SAMPLE" {
		return nil
	}

	report := &TPCIReport{
		Type: msgType,
		Raw:  response,
	}
	idx := 1

	// Check for channel index
	if val, err := strconv.Atoi(parts[idx]); err == nil {
		report.Channel = val
		idx++
	}

	if idx >= len(parts) {
		return report
	}

	// For REP, parts[idx] is the param name
	// For SAMPLE, parts[idx] could be "ALL" or a param name
	report.Param = parts[idx]

	// Extract value (everything after the param/msgType+idx)
	// We look for the literal string after the parameter name/ALL to preserve spaces/braces
	valStart := strings.Index(trimmed, report.Param) + len(report.Param)
	value := strings.TrimSpace(trimmed[valStart:])

	// Remove braces and trailing padding spaces
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		value = strings.Trim(value, "{}")
		value = strings.TrimSpace(value)
	}

	report.Value = value
	return report
}

// Common Axient Commands

func GetAllCommand(channel int) string {
	return NewShureCommand("GET").WithIndex(channel).WithParam("ALL", nil).Build()
}

func SetFlashCommand(channel int, on bool) string {
	return NewShureCommand("SET").WithIndex(channel).WithParam("FLASH", on).Build()
}

func SetMuteCommand(channel int, mute bool) string {
	return NewShureCommand("SET").WithIndex(channel).WithParam("AUDIO_MUTE", mute).Build()
}

// NewAudioGain creates an AxientGain from an integer TPCI value
func NewAudioGain(v int) AxientGain { return AxientGain(v) }

// ParseDeviceStatus parses a device status response
func ParseDeviceStatus(response string) (*DeviceStatus, error) {
	if response == "" || !strings.HasPrefix(response, "REP") {
		return nil, fmt.Errorf("invalid response format")
	}

	trimmed := strings.TrimPrefix(response, "REP ")
	parts := strings.Split(trimmed, ",")

	status := &DeviceStatus{}

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		switch key {
		case "device":
			status.DeviceID = value
		case "gain":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				status.Gain = v
			}
		case "mute":
			if v, err := strconv.Atoi(value); err == nil {
				status.Muted = v == 1
			}
		case "frequency":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				status.Frequency = v
			}
		case "channel":
			if v, err := strconv.Atoi(value); err == nil {
				status.Channel = v
			}
		case "battery":
			if v, err := strconv.Atoi(value); err == nil {
				status.Battery = v
			}
		case "temp":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				status.Temp = v
			}
		}
	}

	return status, nil
}

// DeviceStatus represents the status of a Shure device
type DeviceStatus struct {
	DeviceID  string
	Gain      float64
	Muted     bool
	Frequency float64
	Channel   int
	Battery   int
	Temp      float64
}

// MicOnCommand represents a command to turn on a microphone
type MicOnCommand struct {
	DeviceID DeviceID
}

func (c MicOnCommand) String() string {
	return NewShureCommand("SET").
		WithParam("device", c.DeviceID.String()).
		WithParam("mute", false).
		Build()
}

// MicOffCommand represents a command to turn off a microphone
type MicOffCommand struct {
	DeviceID DeviceID
}

func (c MicOffCommand) String() string {
	return NewShureCommand("SET").
		WithParam("device", c.DeviceID.String()).
		WithParam("mute", true).
		Build()
}

// SetGainCommand represents a command to set the gain
type SetGainCommand struct {
	DeviceID DeviceID
	Gain     Gain
}

func (c SetGainCommand) String() string {
	return NewShureCommand("SET").
		WithParam("device", c.DeviceID.String()).
		WithParam("gain", c.Gain).
		Build()
}

// SetFrequencyCommand represents a command to set the frequency
type SetFrequencyCommand struct {
	DeviceID  DeviceID
	Frequency Frequency
}

func (c SetFrequencyCommand) String() string {
	return NewShureCommand("SET").
		WithParam("device", c.DeviceID.String()).
		WithParam("frequency", c.Frequency).
		Build()
}

// GetStatusCommand represents a command to get device status
type GetStatusCommand struct {
	DeviceID DeviceID
}

func (c GetStatusCommand) String() string {
	return NewShureCommand("GET").
		WithParam("device", c.DeviceID.String()).
		Build()
}

// GetModelCommand requests the model name of the receiver
type GetModelCommand struct {
	Channel int
}

func (c GetModelCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("MODEL", nil).Build()
}

// GetFWVersionCommand requests the firmware version
type GetFWVersionCommand struct {
	Channel int
}

func (c GetFWVersionCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("FW_VER", nil).Build()
}

// GetGroupChannelCommand requests the group and channel settings
type GetGroupChannelCommand struct {
	Channel int
}

func (c GetGroupChannelCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("GROUP_CHANNEL", nil).Build()
}

// SetGroupChannelCommand sets the group and channel (format: gg,cc)
type SetGroupChannelCommand struct {
	Channel    int
	Group      int
	ChannelNum int
}

func (c SetGroupChannelCommand) String() string {
	groupChan := fmt.Sprintf("%02d,%02d", c.Group, c.ChannelNum)
	return NewShureCommand("SET").WithIndex(c.Channel).WithParam("GROUP_CHANNEL", groupChan).Build()
}

// EncryptionMode represents encryption state
type EncryptionMode string

const (
	EncryptionOn     EncryptionMode = "ON"
	EncryptionOff    EncryptionMode = "OFF"
	EncryptionAuto   EncryptionMode = "AUTO"
	EncryptionManual EncryptionMode = "MANUAL"
)

// GetEncryptionModeCommand requests the encryption mode
type GetEncryptionModeCommand struct {
	Channel int
}

func (c GetEncryptionModeCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("ENCRYPTION_MODE", nil).Build()
}

// SetEncryptionModeCommand sets the encryption mode
type SetEncryptionModeCommand struct {
	Channel int
	Mode    EncryptionMode
}

func (c SetEncryptionModeCommand) String() string {
	return NewShureCommand("SET").WithIndex(c.Channel).WithParam("ENCRYPTION_MODE", c.Mode).Build()
}

// SetMeterRateCommand configures the metering rate (in milliseconds)
type SetMeterRateCommand struct {
	Channel int
	RateMs  int
}

func (c SetMeterRateCommand) String() string {
	return NewShureCommand("SET").WithIndex(c.Channel).WithParam("METER_RATE", c.RateMs).Build()
}

// GetMeterRateCommand requests the current meter rate
type GetMeterRateCommand struct {
	Channel int
}

func (c GetMeterRateCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("METER_RATE", nil).Build()
}

// SlotStatus represents transmitter slot status
type SlotStatus string

const (
	SlotEmpty          SlotStatus = "EMPTY"
	SlotStandard       SlotStatus = "STANDARD"
	SlotLinkedInactive SlotStatus = "LINKED.INACTIVE"
	SlotLinkedActive   SlotStatus = "LINKED.ACTIVE"
)

// SlotProperty represents which slot property to query/set
type SlotProperty string

const (
	SlotPropertyStatus         SlotProperty = "SLOT_STATUS"
	SlotPropertyTXDeviceID     SlotProperty = "SLOT_TX_DEVICE_ID"
	SlotPropertyTXModel        SlotProperty = "SLOT_TX_MODEL"
	SlotPropertyShowlinkStatus SlotProperty = "SLOT_SHOWLINK_STATUS"
	SlotPropertyRFPower        SlotProperty = "SLOT_RF_POWER"
	SlotPropertyRFPowerMode    SlotProperty = "SLOT_RF_POWER_MODE"
	SlotPropertyBattBars       SlotProperty = "SLOT_BATT_BARS"
	SlotPropertyBattCharge     SlotProperty = "SLOT_BATT_CHARGE_PERCENT"
	SlotPropertyBattMins       SlotProperty = "SLOT_BATT_MINS"
	SlotPropertyInputPad       SlotProperty = "SLOT_INPUT_PAD"
	SlotPropertyOffset         SlotProperty = "SLOT_OFFSET"
	SlotPropertyPolarity       SlotProperty = "SLOT_POLARITY"
)

// GetSlotPropertyCommand queries a slot property
type GetSlotPropertyCommand struct {
	Channel  int
	Slot     int
	Property SlotProperty
}

func (c GetSlotPropertyCommand) String() string {
	return NewShureCommand("GET").
		WithIndex(c.Channel).
		WithParam(string(c.Property), c.Slot).
		Build()
}

// SetSlotPropertyCommand sets a slot property
type SetSlotPropertyCommand struct {
	Channel  int
	Slot     int
	Property SlotProperty
	Value    interface{}
}

func (c SetSlotPropertyCommand) String() string {
	return NewShureCommand("SET").
		WithIndex(c.Channel).
		WithParam(string(c.Property), c.Value).
		WithParam("slot", c.Slot).
		Build()
}

// QuadversityMode represents quadversity configuration
type QuadversityMode string

const (
	QuadversityOff QuadversityMode = "OFF"
	QuadversityOn  QuadversityMode = "ON"
)

// FDMode represents Frequency Diversity mode
type FDMode string

const (
	FDModeOff     FDMode = "OFF"
	FDModeCombine FDMode = "FD-C"
	FDModeSelect  FDMode = "FD-S"
)

// GetQuadversityModeCommand queries quadversity mode
type GetQuadversityModeCommand struct {
	Channel int
}

func (c GetQuadversityModeCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("QUADVERSITY_MODE", nil).Build()
}

// SetQuadversityModeCommand sets quadversity mode
type SetQuadversityModeCommand struct {
	Channel int
	Mode    QuadversityMode
}

func (c SetQuadversityModeCommand) String() string {
	return NewShureCommand("SET").WithIndex(c.Channel).WithParam("QUADVERSITY_MODE", c.Mode).Build()
}

// GetFDModeCommand queries Frequency Diversity mode
type GetFDModeCommand struct {
	Channel int
}

func (c GetFDModeCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("FD_MODE", nil).Build()
}

// SetFDModeCommand sets Frequency Diversity mode
type SetFDModeCommand struct {
	Channel int
	Mode    FDMode
}

func (c SetFDModeCommand) String() string {
	return NewShureCommand("SET").WithIndex(c.Channel).WithParam("FD_MODE", c.Mode).Build()
}

// GetInterferenceStatusCommand queries interference detection
type GetInterferenceStatusCommand struct {
	Channel int
}

func (c GetInterferenceStatusCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("INTERFERENCE_STATUS", nil).Build()
}

// InterferenceStatus represents interference detection result
type InterferenceStatus string

const (
	InterferenceNone     InterferenceStatus = "NONE"
	InterferenceDetected InterferenceStatus = "DETECTED"
)

// BatteryType represents the type of battery
type BatteryType string

const (
	BatteryAlka BatteryType = "ALKA"
	BatteryLion BatteryType = "LION"
	BatteryNimh BatteryType = "NIMH"
	BatteryLith BatteryType = "LITH"
	BatteryUnkn BatteryType = "UNKN"
)

// TXPowerLevel represents transmitter RF power level (mW for AD)
type TXPowerLevel int

func (p TXPowerLevel) ToMW() int { return int(p) }

// TXOffset represents transmitter offset (actual = value - 12)
type TXOffset int

func (o TXOffset) ToDB() int { return int(o) - 12 }

// TXLock represents transmitter lock status
type TXLock string

const (
	TXLockOn   TXLock = "ON"
	TXLockOff  TXLock = "OFF"
	TXLockAll  TXLock = "ALL"
	TXLockMenu TXLock = "MENU"
)

// GetTXBatteryBarsCommand queries transmitter battery bars
type GetTXBatteryBarsCommand struct {
	Channel int
}

func (c GetTXBatteryBarsCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_BATT_BARS", nil).Build()
}

// GetTXBatteryChargePercentCommand queries transmitter charge percentage
type GetTXBatteryChargePercentCommand struct {
	Channel int
}

func (c GetTXBatteryChargePercentCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_BATT_CHARGE_PERCENT", nil).Build()
}

// GetTXBatteryMinsCommand queries transmitter remaining runtime
type GetTXBatteryMinsCommand struct {
	Channel int
}

func (c GetTXBatteryMinsCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_BATT_MINS", nil).Build()
}

// GetTXBatteryTempCCommand queries transmitter battery temperature in Celsius
type GetTXBatteryTempCCommand struct {
	Channel int
}

func (c GetTXBatteryTempCCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_BATT_TEMP_C", nil).Build()
}

// GetTXBatteryCycleCountCommand queries transmitter battery cycle count
type GetTXBatteryCycleCountCommand struct {
	Channel int
}

func (c GetTXBatteryCycleCountCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_BATT_CYCLE_COUNT", nil).Build()
}

// GetTXBatteryHealthCommand queries transmitter battery health percentage
type GetTXBatteryHealthCommand struct {
	Channel int
}

func (c GetTXBatteryHealthCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_BATT_HEALTH_PERCENT", nil).Build()
}

// GetTXBatteryTypeCommand queries transmitter battery type
type GetTXBatteryTypeCommand struct {
	Channel int
}

func (c GetTXBatteryTypeCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_BATT_TYPE", nil).Build()
}

// GetTXModelCommand queries the transmitter model
type GetTXModelCommand struct {
	Channel int
}

func (c GetTXModelCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_MODEL", nil).Build()
}

// GetTXPowerLevelCommand queries the transmitter RF power level
type GetTXPowerLevelCommand struct {
	Channel int
}

func (c GetTXPowerLevelCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_POWER_LEVEL", nil).Build()
}

// GetTXOffsetCommand queries the transmitter offset
type GetTXOffsetCommand struct {
	Channel int
}

func (c GetTXOffsetCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_OFFSET", nil).Build()
}

// GetTXLockCommand queries transmitter lock status
type GetTXLockCommand struct {
	Channel int
}

func (c GetTXLockCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_LOCK", nil).Build()
}

// SampleReport represents a parsed Axient Digital SAMPLE response with full metering data
type SampleReport struct {
	Channel        int
	Quality        int
	AudioLEDBitmap int
	AudioLevelPeak int
	AudioLevelRMS  int
	RFAntStatus    string
	RFBitmapA      int
	RFRSSI_A       int
	RFBitmapB      int
	RFRSSI_B       int
	RFBitmapC      int
	RFRSSI_C       int
	RFBitmapD      int
	RFRSSI_D       int
	RFBitmapF1     int
	RFRSSI_F1      int
	RFBitmapF2     int
	RFRSSI_F2      int
	Raw            string
}

// AudioLevelPeakDBFS converts peak level to dBFS (offset by 120)
func (s SampleReport) AudioLevelPeakDBFS() int { return s.AudioLevelPeak - 120 }

// AudioLevelRMSDBFS converts RMS level to dBFS (offset by 120)
func (s SampleReport) AudioLevelRMSDBFS() int { return s.AudioLevelRMS - 120 }

// RFRSSI_A_DBM converts RF RSSI to dBm (offset by 128)
func (s SampleReport) RFRSSI_A_DBM() int { return s.RFRSSI_A - 128 }

// RFRSSI_B_DBM converts RF RSSI to dBm (offset by 128)
func (s SampleReport) RFRSSI_B_DBM() int { return s.RFRSSI_B - 128 }

// RFRSSI_C_DBM converts RF RSSI to dBm (offset by 128)
func (s SampleReport) RFRSSI_C_DBM() int { return s.RFRSSI_C - 128 }

// RFRSSI_D_DBM converts RF RSSI to dBm (offset by 128)
func (s SampleReport) RFRSSI_D_DBM() int { return s.RFRSSI_D - 128 }

// RFRSSI_F1_DBM converts RF RSSI F1 to dBm (offset by 128)
func (s SampleReport) RFRSSI_F1_DBM() int { return s.RFRSSI_F1 - 128 }

// RFRSSI_F2_DBM converts RF RSSI F2 to dBm (offset by 128)
func (s SampleReport) RFRSSI_F2_DBM() int { return s.RFRSSI_F2 - 128 }

// ParseSampleReport parses the Axient Digital SAMPLE string format:
// < SAMPLE ch ALL qual audBitmap audPeak audRms rfAntStats rfBitmapA rfRssiA rfBitmapB rfRssiB >
// For Quadversity adds: rfBitmapC rfRssiC rfBitmapD rfRssiD
// For Frequency Diversity adds: rfBitmapF1 rfRssiF1 rfBitmapF2 rfRssiF2
func ParseSampleReport(response string) *SampleReport {
	report := &SampleReport{Raw: response}

	trimmed := strings.Trim(response, "<> ")
	parts := strings.Fields(trimmed)

	if len(parts) < 9 || parts[0] != "SAMPLE" {
		return nil
	}

	if val, err := strconv.Atoi(parts[1]); err == nil {
		report.Channel = val
	}

	if val, err := strconv.ParseInt(parts[2], 16, 64); err == nil {
		report.Quality = int(val)
	}

	if val, err := strconv.ParseInt(parts[3], 16, 64); err == nil {
		report.AudioLEDBitmap = int(val)
	}

	if val, err := strconv.Atoi(parts[4]); err == nil {
		report.AudioLevelPeak = val
	}

	if val, err := strconv.Atoi(parts[5]); err == nil {
		report.AudioLevelRMS = val
	}

	report.RFAntStatus = parts[6]

	if len(parts) > 7 {
		if val, err := strconv.ParseInt(parts[7], 16, 64); err == nil {
			report.RFBitmapA = int(val)
		}
	}
	if len(parts) > 8 {
		if val, err := strconv.Atoi(parts[8]); err == nil {
			report.RFRSSI_A = val
		}
	}
	if len(parts) > 9 {
		if val, err := strconv.ParseInt(parts[9], 16, 64); err == nil {
			report.RFBitmapB = int(val)
		}
	}
	if len(parts) > 10 {
		if val, err := strconv.Atoi(parts[10]); err == nil {
			report.RFRSSI_B = val
		}
	}

	if len(parts) > 11 {
		if val, err := strconv.ParseInt(parts[11], 16, 64); err == nil {
			report.RFBitmapC = int(val)
		}
	}
	if len(parts) > 12 {
		if val, err := strconv.Atoi(parts[12]); err == nil {
			report.RFRSSI_C = val
		}
	}

	if len(parts) > 13 {
		if val, err := strconv.ParseInt(parts[13], 16, 64); err == nil {
			report.RFBitmapD = int(val)
		}
	}
	if len(parts) > 14 {
		if val, err := strconv.Atoi(parts[14]); err == nil {
			report.RFRSSI_D = val
		}
	}

	if len(parts) > 15 {
		if val, err := strconv.ParseInt(parts[15], 16, 64); err == nil {
			report.RFBitmapF1 = int(val)
		}
	}
	if len(parts) > 16 {
		if val, err := strconv.Atoi(parts[16]); err == nil {
			report.RFRSSI_F1 = val
		}
	}

	if len(parts) > 17 {
		if val, err := strconv.ParseInt(parts[17], 16, 64); err == nil {
			report.RFBitmapF2 = int(val)
		}
	}
	if len(parts) > 18 {
		if val, err := strconv.Atoi(parts[18]); err == nil {
			report.RFRSSI_F2 = val
		}
	}

	return report
}

// RepReport represents a parsed REP response with full device/channel properties
type RepReport struct {
	Channel   int
	Param     string
	Value     string
	Raw       string
	DeviceID  string
	Model     string
	FWVersion string
	Gain      AxientGain
	Muted     bool
	Frequency string
	Group     int
	Chan      int
}

// ParseRepReport parses a REP response into structured data
func ParseRepReport(response string) *RepReport {
	report := &RepReport{Raw: response}

	trimmed := strings.Trim(response, "<> ")
	parts := strings.Fields(trimmed)

	if len(parts) < 2 || parts[0] != "REP" {
		return nil
	}

	idx := 1

	if val, err := strconv.Atoi(parts[idx]); err == nil {
		report.Channel = val
		idx++
	}

	if idx >= len(parts) {
		return report
	}

	report.Param = parts[idx]
	idx++

	valStart := strings.Index(trimmed, report.Param) + len(report.Param)
	value := strings.TrimSpace(trimmed[valStart:])

	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		value = strings.Trim(value, "{}")
		value = strings.TrimSpace(value)
	}

	report.Value = value

	switch report.Param {
	case "DEVICE_ID":
		report.DeviceID = value
	case "MODEL":
		report.Model = value
	case "FW_VER":
		report.FWVersion = strings.TrimSuffix(value, "*")
	case "CHAN_NAME", "CHAN NAME":
	case "AUDIO_GAIN", "AUDIO GAIN":
		if val, err := strconv.Atoi(value); err == nil {
			report.Gain = AxientGain(val)
		}
	case "AUDIO_MUTE", "AUDIO MUTE":
		report.Muted = value == "ON" || value == "1"
	case "FREQUENCY":
		report.Frequency = value
	case "GROUP_CHANNEL", "GROUP CHAN":
		value = strings.ReplaceAll(value, " ", "")
		if len(value) >= 5 {
			if g, err := strconv.Atoi(value[:2]); err == nil {
				report.Group = g
			}
			if c, err := strconv.Atoi(value[3:5]); err == nil {
				report.Chan = c
			}
		}
	case "TX_BATT_BARS", "TX BATT BARS":
	case "TX_BATT_CHARGE_PERCENT", "TX BATT CHARGE":
	case "TX_BATT_MINS", "TX BATT MINS":
	case "TX_BATT_TEMP_C", "TX BATT TEMP C":
	case "TX_BATT_CYCLE_COUNT", "TX BATT CYCLE":
	case "TX_BATT_HEALTH_PERCENT", "TX BATT HEALTH":
	case "TX_BATT_TYPE", "TX BATT TYPE":
	case "TX_MODEL", "TX TYPE":
	case "TX_POWER_LEVEL":
	case "TX_OFFSET":
	case "TX_LOCK", "LOCK_STATUS":
	case "ENCRYPTION_MODE", "ENCRYPTION":
	case "INTERFERENCE_STATUS":
	case "QUADVERSITY_MODE":
	case "FD_MODE":
	}

	return report
}

// ============== ULX-D/QLX-D Specific Commands ==============

// HighDensityMode represents ULX-D high density mode
type HighDensityMode bool

const (
	HighDensityOn  HighDensityMode = true
	HighDensityOff HighDensityMode = false
)

// AudioSummingMode represents ULX-D audio summing configuration
type AudioSummingMode string

const (
	AudioSummingOff    AudioSummingMode = "OFF"
	AudioSumming1Plus2 AudioSummingMode = "1+2"
	AudioSumming3Plus4 AudioSummingMode = "3+4"
	AudioSumming12_34  AudioSummingMode = "1+2/3+4"
	AudioSummingAll    AudioSummingMode = "1+2+3+4"
)

// FrequencyDiversityMode represents ULX-D frequency diversity configuration
type FrequencyDiversityMode string

const (
	FreqDiversityOff   FrequencyDiversityMode = "OFF"
	FreqDiversity1p2   FrequencyDiversityMode = "1+2"
	FreqDiversity3p4   FrequencyDiversityMode = "3+4"
	FreqDiversity12_34 FrequencyDiversityMode = "1+2/3+4"
)

// EncryptionWarning represents encryption warning state
type EncryptionWarning bool

const (
	EncryptionWarningOn  EncryptionWarning = true
	EncryptionWarningOff EncryptionWarning = false
)

// TXMuteStatus represents transmitter mute status
type TXMuteStatus string

const (
	TXMuteUnknown TXMuteStatus = "UNKN"
	TXMuteOn      TXMuteStatus = "ON"
	TXMuteOff     TXMuteStatus = "OFF"
)

// TXMuteButtonStatus represents transmitter mute button state
type TXMuteButtonStatus string

const (
	TXMuteButtonUnknown  TXMuteButtonStatus = "UNKN"
	TXMuteButtonPressed  TXMuteButtonStatus = "PRESSED"
	TXMuteButtonReleased TXMuteButtonStatus = "RELEASED"
)

// TXPowerSource represents transmitter power source
type TXPowerSource string

const (
	TXPowerBattery  TXPowerSource = "BATTERY"
	TXPowerExternal TXPowerSource = "EXTERNAL"
	TXPowerUnknown  TXPowerSource = "UNKN"
)

// AntennaSquelchStatus represents antenna status for ULX-D/QLX-D
type AntennaSquelchStatus string

const (
	AntennaAOn     AntennaSquelchStatus = "AX"
	AntennaBOn     AntennaSquelchStatus = "XB"
	AntennaBothOff AntennaSquelchStatus = "XX"
)

// GetHighDensityModeCommand queries high density mode (ULX-D only)
type GetHighDensityModeCommand struct {
	Channel int
}

func (c GetHighDensityModeCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("HIGH_DENSITY", nil).Build()
}

// SetHighDensityModeCommand sets high density mode (ULX-D only)
type SetHighDensityModeCommand struct {
	Channel int
	Mode    HighDensityMode
}

func (c SetHighDensityModeCommand) String() string {
	return NewShureCommand("SET").WithIndex(c.Channel).WithParam("HIGH_DENSITY", c.Mode).Build()
}

// GetAudioSummingModeCommand queries audio summing mode (ULX-D only)
type GetAudioSummingModeCommand struct {
	Channel int
}

func (c GetAudioSummingModeCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("AUDIO_SUMMING_MODE", nil).Build()
}

// SetAudioSummingModeCommand sets audio summing mode (ULX-D only)
type SetAudioSummingModeCommand struct {
	Channel int
	Mode    AudioSummingMode
}

func (c SetAudioSummingModeCommand) String() string {
	return NewShureCommand("SET").WithIndex(c.Channel).WithParam("AUDIO_SUMMING_MODE", c.Mode).Build()
}

// GetFrequencyDiversityModeCommand queries frequency diversity mode (ULX-D only)
type GetFrequencyDiversityModeCommand struct {
	Channel int
}

func (c GetFrequencyDiversityModeCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("FREQUENCY_DIVERSITY_MODE", nil).Build()
}

// SetFrequencyDiversityModeCommand sets frequency diversity mode (ULX-D only)
type SetFrequencyDiversityModeCommand struct {
	Channel int
	Mode    FrequencyDiversityMode
}

func (c SetFrequencyDiversityModeCommand) String() string {
	return NewShureCommand("SET").WithIndex(c.Channel).WithParam("FREQUENCY_DIVERSITY_MODE", c.Mode).Build()
}

// GetEncryptionWarningCommand queries encryption warning state (ULX-D only)
type GetEncryptionWarningCommand struct {
	Channel int
}

func (c GetEncryptionWarningCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("ENCRYPTION_WARNING", nil).Build()
}

// SetEncryptionWarningCommand sets encryption warning state (ULX-D only)
type SetEncryptionWarningCommand struct {
	Channel int
	Mode    EncryptionWarning
}

func (c SetEncryptionWarningCommand) String() string {
	return NewShureCommand("SET").WithIndex(c.Channel).WithParam("ENCRYPTION_WARNING", c.Mode).Build()
}

// GetTXMuteStatusCommand queries transmitter mute status (ULX-D only)
type GetTXMuteStatusCommand struct {
	Channel int
}

func (c GetTXMuteStatusCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_MUTE_STATUS", nil).Build()
}

// GetTXMuteButtonStatusCommand queries transmitter mute button status (ULX-D only)
type GetTXMuteButtonStatusCommand struct {
	Channel int
}

func (c GetTXMuteButtonStatusCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_MUTE_BUTTON_STATUS", nil).Build()
}

// GetTXPowerSourceCommand queries transmitter power source (ULX-D only)
type GetTXPowerSourceCommand struct {
	Channel int
}

func (c GetTXPowerSourceCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("TX_POWER_SOURCE", nil).Build()
}

// ============== SLX-D/SLX-D+ Specific Commands ==============

// AudioOutputLevelSwitch represents SLX-D audio output level
type AudioOutputLevelSwitch string

const (
	AudioOutputMic  AudioOutputLevelSwitch = "MIC"
	AudioOutputLine AudioOutputLevelSwitch = "LINE"
)

// RemotePairStatus represents Bluetooth remote pairing status
type RemotePairStatus string

const (
	RemotePairOn      RemotePairStatus = "ON"
	RemotePairOff     RemotePairStatus = "OFF"
	RemotePairRequest RemotePairStatus = "REQUEST"
)

// RemotePairAction represents accept/reject action for remote pairing
type RemotePairAction string

const (
	RemotePairAccept RemotePairAction = "ACCEPT"
	RemotePairReject RemotePairAction = "REJECT"
)

// LinkTXStatus represents linked transmitter status for SLX-D+
type LinkTXStatus string

const (
	LinkTXEmpty          LinkTXStatus = "EMPTY"
	LinkTXLinkedInactive LinkTXStatus = "LINKED.INACTIVE"
	LinkTXLinkedActive   LinkTXStatus = "LINKED.ACTIVE"
)

// EncryptionStatus represents encryption status for SLX-D/SLX-D+
type EncryptionStatus string

const (
	EncryptionOK    EncryptionStatus = "OK"
	EncryptionError EncryptionStatus = "ERROR"
)

// NetworkInterface represents network interface selection
type NetworkInterface string

const (
	NetworkInterfaceSC NetworkInterface = "SC"
	NetworkInterfaceD1 NetworkInterface = "D1"
	NetworkInterfaceD2 NetworkInterface = "D2"
)

// IPMode represents IP address mode
type IPMode string

const (
	IPModeAuto   IPMode = "AUTO"
	IPModeManual IPMode = "MANUAL"
)

// GetAudioOutputLevelSwitchCommand queries audio output level switch (SLX-D/SLX-D+)
type GetAudioOutputLevelSwitchCommand struct {
	Channel int
}

func (c GetAudioOutputLevelSwitchCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("AUDIO_OUT_LVL_SWITCH", nil).Build()
}

// SetAudioOutputLevelSwitchCommand sets audio output level switch (SLX-D/SLX-D+)
type SetAudioOutputLevelSwitchCommand struct {
	Channel int
	Level   AudioOutputLevelSwitch
}

func (c SetAudioOutputLevelSwitchCommand) String() string {
	return NewShureCommand("SET").WithIndex(c.Channel).WithParam("AUDIO_OUT_LVL_SWITCH", c.Level).Build()
}

// ============== SLX-D+ Dante Network Commands ==============

// GetNADeviceNameCommand queries Dante device name (SLX-D+ only)
type GetNADeviceNameCommand struct {
}

func (c GetNADeviceNameCommand) String() string {
	return NewShureCommand("GET").WithParam("NA_DEVICE_NAME", nil).Build()
}

// GetNAChannelNameCommand queries Dante channel name (SLX-D+ only)
type GetNAChannelNameCommand struct {
	Channel int
}

func (c GetNAChannelNameCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("NA_CHAN_NAME", nil).Build()
}

// SetNetSettingsCommand configures network settings (SLX-D+ only)
type SetNetSettingsCommand struct {
	Interface  NetworkInterface
	IPMode     IPMode
	IPAddress  string
	SubnetMask string
	Gateway    string
}

func (c SetNetSettingsCommand) String() string {
	return NewShureCommand("SET").
		WithParam("NET_SETTINGS", []string{
			string(c.Interface),
			string(c.IPMode),
			c.IPAddress,
			c.SubnetMask,
			c.Gateway,
		}).Build()
}

// GetAppConnEnabledCommand queries app connectivity (SLX-D+ only)
type GetAppConnEnabledCommand struct {
}

func (c GetAppConnEnabledCommand) String() string {
	return NewShureCommand("GET").WithParam("APP_CONN_ENABLED", nil).Build()
}

// SetRemotePairCommand sets Bluetooth remote pairing (SLX-D+ only)
type SetRemotePairCommand struct {
	Channel int
	Status  RemotePairStatus
}

func (c SetRemotePairCommand) String() string {
	return NewShureCommand("SET").
		WithIndex(c.Channel).
		WithParam("REM_PAIR", c.Status).
		Build()
}

// RespondRemotePairCommand responds to a remote pairing request (SLX-D+ only)
type RespondRemotePairCommand struct {
	Channel int
	TxName  string
	Action  RemotePairAction
}

func (c RespondRemotePairCommand) String() string {
	return NewShureCommand("SET").
		WithIndex(c.Channel).
		WithParam("REM_PAIR", c.Action).
		WithParam("TxName", c.TxName).
		Build()
}

// GetLinkStatusCommand queries linked transmitter status (SLX-D+ only)
type GetLinkStatusCommand struct {
	Channel int
	Slot    int
}

func (c GetLinkStatusCommand) String() string {
	return NewShureCommand("GET").
		WithIndex(c.Channel).
		WithParam("LINK_STATUS", c.Slot).
		Build()
}

// GetLinkTXModelCommand queries linked transmitter model (SLX-D+ only)
type GetLinkTXModelCommand struct {
	Channel int
	Slot    int
}

func (c GetLinkTXModelCommand) String() string {
	return NewShureCommand("GET").
		WithIndex(c.Channel).
		WithParam("LINK_TX_MODEL", c.Slot).
		Build()
}

// GetLinkTXBatteryMinsCommand queries linked transmitter battery runtime (SLX-D+ only)
type GetLinkTXBatteryMinsCommand struct {
	Channel int
	Slot    int
}

func (c GetLinkTXBatteryMinsCommand) String() string {
	return NewShureCommand("GET").
		WithIndex(c.Channel).
		WithParam("LINK_TX_BATT_MINS", c.Slot).
		Build()
}

// RebootLinkTXCommand triggers remote reboot of linked transmitter (SLX-D+ only)
type RebootLinkTXCommand struct {
	Channel int
	Slot    int
}

func (c RebootLinkTXCommand) String() string {
	return NewShureCommand("SET").
		WithIndex(c.Channel).
		WithParam("LINK_TX_REBOOT", c.Slot).
		Build()
}

// GetEncryptionStatusCommand queries encryption status (SLX-D/SLX-D+)
type GetEncryptionStatusCommand struct {
	Channel int
}

func (c GetEncryptionStatusCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("ENCRYPTION_STATUS", nil).Build()
}

// GetSLXInterferenceStatusCommand queries interference detection (SLX-D/SLX-D+)
type GetSLXInterferenceStatusCommand struct {
	Channel int
}

func (c GetSLXInterferenceStatusCommand) String() string {
	return NewShureCommand("GET").WithIndex(c.Channel).WithParam("INTERFERENCE_STATUS", nil).Build()
}

// ============== Networked Charger Commands (ULX-D) ==============

// GetTXAvailableCommand queries if transmitter is available in charger bay
type GetTXAvailableCommand struct {
	Bay int
}

func (c GetTXAvailableCommand) String() string {
	return NewShureCommand("GET").
		WithIndex(c.Bay).
		WithParam("TX_AVAILABLE", nil).
		Build()
}

// GetBattTimeToFullCommand queries battery time to full charge
type GetBattTimeToFullCommand struct {
	Bay int
}

func (c GetBattTimeToFullCommand) String() string {
	return NewShureCommand("GET").
		WithIndex(c.Bay).
		WithParam("BATT_TIME_TO_FULL", nil).
		Build()
}

// GetBattBarsCommand queries battery bars on charger
type GetBattBarsCommand struct {
	Bay int
}

func (c GetBattBarsCommand) String() string {
	return NewShureCommand("GET").
		WithIndex(c.Bay).
		WithParam("BATT_BARS", nil).
		Build()
}

// GetBattChargeCommand queries battery charge percentage on charger
type GetBattChargeCommand struct {
	Bay int
}

func (c GetBattChargeCommand) String() string {
	return NewShureCommand("GET").
		WithIndex(c.Bay).
		WithParam("BATT_CHARGE", nil).
		Build()
}

// GetBattCycleCommand queries battery cycle count on charger
type GetBattCycleCommand struct {
	Bay int
}

func (c GetBattCycleCommand) String() string {
	return NewShureCommand("GET").
		WithIndex(c.Bay).
		WithParam("BATT_CYCLE", nil).
		Build()
}

// GetBattHealthCommand queries battery health on charger
type GetBattHealthCommand struct {
	Bay int
}

func (c GetBattHealthCommand) String() string {
	return NewShureCommand("GET").
		WithIndex(c.Bay).
		WithParam("BATT_HEALTH", nil).
		Build()
}

// ============== Universal SAMPLE Parsers for All Model Families ==============

// ULXDSampleReport represents a parsed ULX-D/QLX-D SAMPLE response
type ULXDSampleReport struct {
	Channel    int
	AntStatus  AntennaSquelchStatus
	RFLevel    int
	AudioLevel int
	Raw        string
}

func (s ULXDSampleReport) RFLevelDBM() int { return s.RFLevel - 128 }

func (s ULXDSampleReport) AudioLevelDBFS() int { return s.AudioLevel - 50 }

// ParseULXDSampleReport parses the ULX-D/QLX-D SAMPLE string format:
// < SAMPLE 1 ALL nn aaa eee >
// nn = Antenna Squelch Status (AX=A on, XB=B on, XX=Both off)
// aaa = RF Level (000 to 115, subtract 128 for dBm)
// eee = Audio Level (000 to 050)
func ParseULXDSampleReport(response string) *ULXDSampleReport {
	report := &ULXDSampleReport{Raw: response}

	trimmed := strings.Trim(response, "<> ")
	parts := strings.Fields(trimmed)

	if len(parts) < 5 || parts[0] != "SAMPLE" {
		return nil
	}

	if val, err := strconv.Atoi(parts[1]); err == nil {
		report.Channel = val
	}

	report.AntStatus = AntennaSquelchStatus(parts[3])

	if val, err := strconv.Atoi(parts[4]); err == nil {
		report.RFLevel = val
	}

	if len(parts) > 5 {
		if val, err := strconv.Atoi(parts[5]); err == nil {
			report.AudioLevel = val
		}
	}

	return report
}

// SLDXSampleReport represents a parsed SLX-D/SLX-D+ SAMPLE response
type SLDXSampleReport struct {
	Channel   int
	AudioPeak int
	AudioRMS  int
	RFRSSI    int
	Raw       string
}

func (s SLDXSampleReport) AudioPeakDBFS() int { return s.AudioPeak - 120 }

func (s SLDXSampleReport) AudioRMSDBFS() int { return s.AudioRMS - 120 }

func (s SLDXSampleReport) RFRSSIDBM() int { return s.RFRSSI - 120 }

// ParseSLDXSampleReport parses the SLX-D/SLX-D+ SAMPLE string format:
// < SAMPLE ch ALL audPeak audRms rfRssi >
// audPeak & audRms = Audio Levels (000 to 120, subtract 120 for dBFS)
// rfRssi = RF Level (000 to 120, subtract 120 for dBm)
func ParseSLDXSampleReport(response string) *SLDXSampleReport {
	report := &SLDXSampleReport{Raw: response}

	trimmed := strings.Trim(response, "<> ")
	parts := strings.Fields(trimmed)

	if len(parts) < 5 || parts[0] != "SAMPLE" {
		return nil
	}

	if val, err := strconv.Atoi(parts[1]); err == nil {
		report.Channel = val
	}

	if len(parts) > 3 {
		if val, err := strconv.Atoi(parts[3]); err == nil {
			report.AudioPeak = val
		}
	}

	if len(parts) > 4 {
		if val, err := strconv.Atoi(parts[4]); err == nil {
			report.AudioRMS = val
		}
	}

	if len(parts) > 5 {
		if val, err := strconv.Atoi(parts[5]); err == nil {
			report.RFRSSI = val
		}
	}

	return report
}

// DetectSampleFormat attempts to detect the SAMPLE format based on the response
func DetectSampleFormat(response string) string {
	trimmed := strings.Trim(response, "<> ")
	parts := strings.Fields(trimmed)

	if len(parts) < 5 || parts[0] != "SAMPLE" {
		return "unknown"
	}

	if parts[2] == "ALL" {
		// Axient Digital: < SAMPLE ch ALL qual audBitmap audPeak audRms rfAntStats rfBitmapA rfRssiA rfBitmapB rfRssiB >
		// Has hex values for quality and bitmaps (at least 9 parts, parts[3] is hex)
		if len(parts) >= 9 {
			if _, err := strconv.ParseInt(parts[3], 16, 64); err == nil {
				return "axient"
			}
		}

		// ULX-D/QLX-D: < SAMPLE 1 ALL AX 100 030 > - has antenna status like AX, XB, XX
		// Check for antenna status pattern (2 chars, one of A/B/X)
		if len(parts) >= 4 && len(parts[3]) == 2 {
			status := parts[3]
			if (status[0] == 'A' || status[0] == 'X' || status[0] == 'B') &&
				(status[1] == 'X' || status[1] == 'A' || status[1] == 'B') {
				return "ulxd"
			}
		}

		// SLX-D/SLX-D+: < SAMPLE ch ALL audPeak audRms rfRssi >
		// Check if parts are numeric (000-120)
		if len(parts) >= 6 {
			allNum := true
			for i := 3; i < len(parts) && i < 6; i++ {
				if _, err := strconv.Atoi(parts[i]); err != nil {
					allNum = false
					break
				}
			}
			if allNum {
				return "sldx"
			}
		}
	}

	return "unknown"
}
