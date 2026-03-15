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
			case AxientGain:
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

// Domain-specific types for Axient Digital

// AxientGain handles the 18dB offset (TPCI 030 = 12dB)
type AxientGain int

func (g AxientGain) ToDB() int      { return int(g) - 18 }
func (g AxientGain) String() string { return fmt.Sprintf("%03d", int(g)) }

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
	Channel int
	Param   string
	Value   string
	Raw     string
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

	report := &TPCIReport{Raw: response}
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
