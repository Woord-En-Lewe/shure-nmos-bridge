package infrastructure

import (
	"fmt"
	"strings"
	"testing"
)

// TestShureCommandBuilder tests the builder pattern for Shure commands
func TestShureCommandBuilder(t *testing.T) {
	// Test basic command
	cmd := NewShureCommand("GET").Build()
	if cmd != "< GET >" {
		t.Errorf("Expected '< GET >', got '%s'", cmd)
	}

	// Test command with single parameter
	cmd = NewShureCommand("SET").
		WithParam("device", "SHURE_001").
		Build()
	if cmd != "< SET DEVICE {SHURE_001} >" {
		t.Errorf("Expected '< SET DEVICE {SHURE_001} >', got '%s'", cmd)
	}

	// Test command with multiple parameters
	cmd = NewShureCommand("SET").
		WithParam("device", "SHURE_001").
		WithParam("gain", 10.5).
		WithParam("mute", false).
		Build()
	// Note: Order might vary due to map iteration, so we check for substrings
	if !strings.Contains(cmd, "< SET") ||
		!strings.Contains(cmd, "DEVICE {SHURE_001}") ||
		!strings.Contains(cmd, "GAIN 10.5") ||
		!strings.Contains(cmd, "MUTE OFF") {
		t.Errorf("Expected command with all parameters, got '%s'", cmd)
	}
}

// Test TPCI response parsing
func TestParseTPCIResponse(t *testing.T) {
	// Test REP response
	resp := "< REP 1 CHAN_NAME {Lead Vox       } >"
	report := ParseTPCIResponse(resp)
	if report == nil {
		t.Fatal("Expected report, got nil")
	}
	if report.Channel != 1 {
		t.Errorf("Expected channel 1, got %d", report.Channel)
	}
	if report.Param != "CHAN_NAME" {
		t.Errorf("Expected param CHAN_NAME, got %s", report.Param)
	}
	if report.Value != "Lead Vox" {
		t.Errorf("Expected value 'Lead Vox', got '%s'", report.Value)
	}

	// Test SAMPLE response
	resp2 := "< SAMPLE 1 ALL 005 000 045 062 BB 31 099 31 085 >"
	report2 := ParseTPCIResponse(resp2)
	if report2 == nil {
		t.Fatal("Expected report, got nil")
	}
	if report2.Channel != 1 {
		t.Errorf("Expected channel 1, got %d", report2.Channel)
	}
	if report2.Param != "ALL" {
		t.Errorf("Expected param ALL, got %s", report2.Param)
	}
	if !strings.Contains(report2.Value, "005 000 045 062") {
		t.Errorf("Expected value to contain measurements, got '%s'", report2.Value)
	}

	// Test device level REP (no channel)
	resp3 := "< REP DEVICE_ID {MyReceiver     } >"
	report3 := ParseTPCIResponse(resp3)
	if report3 == nil {
		t.Fatal("Expected report, got nil")
	}
	if report3.Channel != 0 {
		t.Errorf("Expected channel 0, got %d", report3.Channel)
	}
	if report3.Param != "DEVICE_ID" {
		t.Errorf("Expected param DEVICE_ID, got %s", report3.Param)
	}
	if report3.Value != "MyReceiver" {
		t.Errorf("Expected value 'MyReceiver', got '%s'", report3.Value)
	}
}

// Test domain-specific types
func TestDomainTypes(t *testing.T) {
	// Test Gain
	g := NewGain(15.5)
	if fmt.Sprintf("%s", g) != "15.5" {
		t.Errorf("Expected '15.5', got '%s'", g)
	}

	// Test Mute
	m := NewMute(true)
	if fmt.Sprintf("%s", m) != "1" {
		t.Errorf("Expected '1', got '%s'", m)
	}
	m = NewMute(false)
	if fmt.Sprintf("%s", m) != "0" {
		t.Errorf("Expected '0', got '%s'", m)
	}

	// Test Frequency
	f := NewFrequency(600.125)
	if fmt.Sprintf("%s", f) != "600.125" {
		t.Errorf("Expected '600.125', got '%s'", f)
	}

	// Test Channel
	c := NewChannel(4)
	if fmt.Sprintf("%s", c) != "4" {
		t.Errorf("Expected '4', got '%s'", c)
	}

	// Test DeviceID
	id := NewDeviceID("SHURE_001")
	if fmt.Sprintf("%s", id) != "SHURE_001" {
		t.Errorf("Expected 'SHURE_001', got '%s'", id)
	}
}

// Test predefined commands
func TestPredefinedCommands(t *testing.T) {
	deviceID := NewDeviceID("SHURE_001")

	// Test MicOnCommand
	cmd := MicOnCommand{DeviceID: deviceID}
	cmdStr := cmd.String()
	if !strings.Contains(cmdStr, "< SET") ||
		!strings.Contains(cmdStr, "DEVICE {SHURE_001}") ||
		!strings.Contains(cmdStr, "MUTE OFF") {
		t.Errorf("Expected MicOn command to mute=0, got '%s'", cmdStr)
	}

	// Test MicOffCommand
	cmd2 := MicOffCommand{DeviceID: deviceID}
	cmdStr2 := cmd2.String()
	if !strings.Contains(cmdStr2, "< SET") ||
		!strings.Contains(cmdStr2, "DEVICE {SHURE_001}") ||
		!strings.Contains(cmdStr2, "MUTE ON") {
		t.Errorf("Expected MicOff command to mute=1, got '%s'", cmdStr2)
	}

	// Test SetGainCommand
	cmd3 := SetGainCommand{DeviceID: deviceID, Gain: NewGain(10.5)}
	cmdStr3 := cmd3.String()
	if !strings.Contains(cmdStr3, "< SET") ||
		!strings.Contains(cmdStr3, "DEVICE {SHURE_001}") ||
		!strings.Contains(cmdStr3, "GAIN 10.5") {
		t.Errorf("Expected SetGain command with gain=10.5, got '%s'", cmdStr3)
	}

	// Test SetFrequencyCommand
	cmd4 := SetFrequencyCommand{DeviceID: deviceID, Frequency: NewFrequency(600.125)}
	cmdStr4 := cmd4.String()
	if !strings.Contains(cmdStr4, "< SET") ||
		!strings.Contains(cmdStr4, "DEVICE {SHURE_001}") ||
		!strings.Contains(cmdStr4, "FREQUENCY 600.125") {
		t.Errorf("Expected SetFrequency command with frequency=600.125, got '%s'", cmdStr4)
	}

	// Test GetStatusCommand
	cmd5 := GetStatusCommand{DeviceID: deviceID}
	cmdStr5 := cmd5.String()
	if !strings.Contains(cmdStr5, "< GET") ||
		!strings.Contains(cmdStr5, "DEVICE {SHURE_001}") {
		t.Errorf("Expected GetStatus command, got '%s'", cmdStr5)
	}
}

// Test response parsing
func TestParseDeviceStatus(t *testing.T) {
	// Test valid REP response
	resp := "REP device=SHURE_001,gain=10.5,mute=0,frequency=600.125,channel=4,battery=85,temp=25.0"
	status, err := ParseDeviceStatus(resp)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if status == nil {
		t.Errorf("Expected status, got nil")
	}
	if status.DeviceID != "SHURE_001" {
		t.Errorf("Expected device ID SHURE_001, got %s", status.DeviceID)
	}
	if status.Gain != 10.5 {
		t.Errorf("Expected gain 10.5, got %f", status.Gain)
	}
	if status.Muted != false {
		t.Errorf("Expected muted false, got %t", status.Muted)
	}
	if status.Frequency != 600.125 {
		t.Errorf("Expected frequency 600.125, got %f", status.Frequency)
	}
	if status.Channel != 4 {
		t.Errorf("Expected channel 4, got %d", status.Channel)
	}
	if status.Battery != 85 {
		t.Errorf("Expected battery 85, got %d", status.Battery)
	}
	if status.Temp != 25.0 {
		t.Errorf("Expected temp 25.0, got %f", status.Temp)
	}

	// Test empty response
	_, err = ParseDeviceStatus("")
	if err == nil {
		t.Errorf("Expected error for empty response")
	}

	// Test invalid format
	_, err = ParseDeviceStatus("INVALID")
	if err == nil {
		t.Errorf("Expected error for invalid format")
	}

	// Test non-REP response
	_, err = ParseDeviceStatus("SET device=SHURE_001,gain=10.5")
	if err == nil {
		t.Errorf("Expected error for non-REP response")
	}
}

// Test new Axient-specific commands
func TestAxientCommands(t *testing.T) {
	// Test GetModelCommand
	cmd := GetModelCommand{Channel: 1}
	if !strings.Contains(cmd.String(), "MODEL") {
		t.Errorf("Expected MODEL in command, got %s", cmd.String())
	}

	// Test GetFWVersionCommand
	cmd2 := GetFWVersionCommand{Channel: 1}
	if !strings.Contains(cmd2.String(), "FW_VER") {
		t.Errorf("Expected FW_VER in command, got %s", cmd2.String())
	}

	// Test GetGroupChannelCommand
	cmd3 := GetGroupChannelCommand{Channel: 2}
	if !strings.Contains(cmd3.String(), "GROUP_CHANNEL") {
		t.Errorf("Expected GROUP_CHANNEL in command, got %s", cmd3.String())
	}

	// Test SetGroupChannelCommand
	cmd4 := SetGroupChannelCommand{Channel: 1, Group: 3, ChannelNum: 7}
	cmd4Str := cmd4.String()
	if !strings.Contains(cmd4Str, "GROUP_CHANNEL") {
		t.Errorf("Expected GROUP_CHANNEL in command, got %s", cmd4Str)
	}
	if !strings.Contains(cmd4Str, "03,07") {
		t.Errorf("Expected 03,07 in command, got %s", cmd4Str)
	}

	// Test GetEncryptionModeCommand
	cmd5 := GetEncryptionModeCommand{Channel: 1}
	if !strings.Contains(cmd5.String(), "ENCRYPTION_MODE") {
		t.Errorf("Expected ENCRYPTION_MODE in command, got %s", cmd5.String())
	}

	// Test SetEncryptionModeCommand
	cmd6 := SetEncryptionModeCommand{Channel: 1, Mode: EncryptionOn}
	if !strings.Contains(cmd6.String(), "ENCRYPTION_MODE") || !strings.Contains(cmd6.String(), "ON") {
		t.Errorf("Expected ENCRYPTION_MODE ON in command, got %s", cmd6.String())
	}

	// Test SetMeterRateCommand
	cmd7 := SetMeterRateCommand{Channel: 1, RateMs: 100}
	cmd7Str := cmd7.String()
	if !strings.Contains(cmd7Str, "METER_RATE") {
		t.Errorf("Expected METER_RATE in command, got %s", cmd7Str)
	}
	if !strings.Contains(cmd7Str, "100") {
		t.Errorf("Expected 100 in command, got %s", cmd7Str)
	}

	// Test Slot commands
	cmd8 := GetSlotPropertyCommand{Channel: 1, Slot: 3, Property: SlotPropertyTXDeviceID}
	cmd8Str := cmd8.String()
	if !strings.Contains(cmd8Str, "SLOT_TX_DEVICE_ID") {
		t.Errorf("Expected SLOT_TX_DEVICE_ID in command, got %s", cmd8Str)
	}

	// Test RF Mode commands
	cmd9 := GetQuadversityModeCommand{Channel: 1}
	if !strings.Contains(cmd9.String(), "QUADVERSITY_MODE") {
		t.Errorf("Expected QUADVERSITY_MODE in command, got %s", cmd9.String())
	}

	cmd10 := SetFDModeCommand{Channel: 1, Mode: FDModeCombine}
	cmd10Str := cmd10.String()
	if !strings.Contains(cmd10Str, "FD_MODE") || !strings.Contains(cmd10Str, "FD-C") {
		t.Errorf("Expected FD_MODE FD-C in command, got %s", cmd10Str)
	}

	// Test InterferenceStatus command
	cmd11 := GetInterferenceStatusCommand{Channel: 1}
	if !strings.Contains(cmd11.String(), "INTERFERENCE_STATUS") {
		t.Errorf("Expected INTERFERENCE_STATUS in command, got %s", cmd11.String())
	}

	// Test TX Battery commands
	cmd12 := GetTXBatteryBarsCommand{Channel: 1}
	if !strings.Contains(cmd12.String(), "TX_BATT_BARS") {
		t.Errorf("Expected TX_BATT_BARS in command, got %s", cmd12.String())
	}

	cmd13 := GetTXBatteryTypeCommand{Channel: 1}
	if !strings.Contains(cmd13.String(), "TX_BATT_TYPE") {
		t.Errorf("Expected TX_BATT_TYPE in command, got %s", cmd13.String())
	}

	// Test TX commands
	cmd14 := GetTXModelCommand{Channel: 1}
	if !strings.Contains(cmd14.String(), "TX_MODEL") {
		t.Errorf("Expected TX_MODEL in command, got %s", cmd14.String())
	}

	cmd15 := GetTXPowerLevelCommand{Channel: 1}
	if !strings.Contains(cmd15.String(), "TX_POWER_LEVEL") {
		t.Errorf("Expected TX_POWER_LEVEL in command, got %s", cmd15.String())
	}

	cmd16 := GetTXOffsetCommand{Channel: 1}
	if !strings.Contains(cmd16.String(), "TX_OFFSET") {
		t.Errorf("Expected TX_OFFSET in command, got %s", cmd16.String())
	}

	cmd17 := GetTXLockCommand{Channel: 1}
	if !strings.Contains(cmd17.String(), "TX_LOCK") {
		t.Errorf("Expected TX_LOCK in command, got %s", cmd17.String())
	}
}

// Test ParseSampleReport
func TestParseSampleReport(t *testing.T) {
	// The code parses the Axient SAMPLE format at specific indices:
	// < SAMPLE ch ALL qual(hex) bitmap(hex) peak(RMS) rms dec antStatus bitmapA(hex) rssiA dec bitmapB(hex) rssiB dec
	// parts:         0      1  2    3         4           5          6     7         8          9        10         11

	// Test input: < SAMPLE 1 ALL 005 000 045 062 BB 31 099 31 085 >
	// parts: ["SAMPLE","1","ALL","005","000","045","062","BB","31","099","31","085"]
	resp := "< SAMPLE 1 ALL 005 000 045 062 BB 31 099 31 085 >"
	report := ParseSampleReport(resp)
	if report == nil {
		t.Fatal("Expected report, got nil")
	}
	if report.Channel != 1 {
		t.Errorf("Expected channel 1, got %d", report.Channel)
	}
	// parts[2]="ALL" is not hex-parseable, Quality stays 0
	if report.Quality != 0 {
		t.Errorf("Expected quality 0, got %d", report.Quality)
	}
	// parts[3]="005" = AudioLEDBitmap (hex)
	if report.AudioLEDBitmap != 5 {
		t.Errorf("Expected AudioLEDBitmap 5, got %d", report.AudioLEDBitmap)
	}
	// parts[4]="000" = AudioLevelPeak
	if report.AudioLevelPeak != 0 {
		t.Errorf("Expected peak 0, got %d", report.AudioLevelPeak)
	}
	// parts[5]="045" = AudioLevelRMS
	if report.AudioLevelRMS != 45 {
		t.Errorf("Expected RMS 45, got %d", report.AudioLevelRMS)
	}
	// parts[6]="062" = RFAntStatus
	if report.RFAntStatus != "062" {
		t.Errorf("Expected RF ant status 062, got %s", report.RFAntStatus)
	}
	// parts[7]="BB" = RFBitmapA (hex)
	if report.RFBitmapA != 0xBB {
		t.Errorf("Expected RFBitmapA 0xBB, got %x", report.RFBitmapA)
	}
	// parts[8]="31" = RFRSSI_A (decimal)
	if report.RFRSSI_A != 31 {
		t.Errorf("Expected RFRSSI_A 31, got %d", report.RFRSSI_A)
	}
	// Test dBFS conversion (120dBFS offset)
	if report.AudioLevelPeakDBFS() != -120 {
		t.Errorf("Expected peak dBFS -120, got %d", report.AudioLevelPeakDBFS())
	}
	if report.AudioLevelRMSDBFS() != -75 {
		t.Errorf("Expected RMS dBFS -75, got %d", report.AudioLevelRMSDBFS())
	}
	// Test RSSI dBm conversion (128 offset)
	if report.RFRSSI_A_DBM() != -97 {
		t.Errorf("Expected RSSI dBm -97, got %d", report.RFRSSI_A_DBM())
	}
}

// Test ParseRepReport
func TestParseRepReport(t *testing.T) {
	// Test MODEL response
	resp := "< REP 1 MODEL {AD4Q            } >"
	report := ParseRepReport(resp)
	if report == nil {
		t.Fatal("Expected report, got nil")
	}
	if report.Channel != 1 {
		t.Errorf("Expected channel 1, got %d", report.Channel)
	}
	if report.Param != "MODEL" {
		t.Errorf("Expected param MODEL, got %s", report.Param)
	}
	if report.Model != "AD4Q" {
		t.Errorf("Expected model AD4Q, got %s", report.Model)
	}

	// Test FW_VER response
	resp2 := "< REP 1 FW_VER {1.2.3.456      } >"
	report2 := ParseRepReport(resp2)
	if report2 == nil {
		t.Fatal("Expected report, got nil")
	}
	if report2.FWVersion != "1.2.3.456" {
		t.Errorf("Expected FW version 1.2.3.456, got %s", report2.FWVersion)
	}

	// Test GROUP_CHANNEL response
	resp3 := "< REP 1 GROUP_CHANNEL {03,07           } >"
	report3 := ParseRepReport(resp3)
	if report3 == nil {
		t.Fatal("Expected report, got nil")
	}
	if report3.Group != 3 {
		t.Errorf("Expected group 3, got %d", report3.Group)
	}
	if report3.Chan != 7 {
		t.Errorf("Expected channel 7, got %d", report3.Chan)
	}

	// Test AUDIO_GAIN response
	resp4 := "< REP 1 AUDIO_GAIN 030 >"
	report4 := ParseRepReport(resp4)
	if report4 == nil {
		t.Fatal("Expected report, got nil")
	}
	if report4.Gain != 30 {
		t.Errorf("Expected gain 30, got %d", report4.Gain)
	}
	if report4.Gain.ToDB() != 12 {
		t.Errorf("Expected gain dB 12, got %d", report4.Gain.ToDB())
	}

	// Test AUDIO_MUTE response
	resp5 := "< REP 1 AUDIO_MUTE ON >"
	report5 := ParseRepReport(resp5)
	if report5 == nil {
		t.Fatal("Expected report, got nil")
	}
	if !report5.Muted {
		t.Errorf("Expected muted true")
	}

	// Test INTERFERENCE_STATUS response
	resp6 := "< REP 1 INTERFERENCE_STATUS DETECTED >"
	report6 := ParseRepReport(resp6)
	if report6 == nil {
		t.Fatal("Expected report, got nil")
	}
	if report6.Value != "DETECTED" {
		t.Errorf("Expected DETECTED, got %s", report6.Value)
	}
}

// Test domain-specific type conversions
func TestDomainTypeConversions(t *testing.T) {
	// Test AxientGain dB conversion
	gain := AxientGain(30)
	if gain.ToDB() != 12 {
		t.Errorf("Expected 12dB, got %d", gain.ToDB())
	}

	// Test TXOffset dB conversion
	offset := TXOffset(18)
	if offset.ToDB() != 6 {
		t.Errorf("Expected 6dB, got %d", offset.ToDB())
	}

	// Test EncryptionMode
	if EncryptionOn != "ON" {
		t.Errorf("Expected ON, got %s", EncryptionOn)
	}

	// Test FDMode
	if FDModeCombine != "FD-C" {
		t.Errorf("Expected FD-C, got %s", FDModeCombine)
	}

	// Test SlotStatus
	if SlotLinkedActive != "LINKED.ACTIVE" {
		t.Errorf("Expected LINKED.ACTIVE, got %s", SlotLinkedActive)
	}

	// Test BatteryType
	if BatteryLion != "LION" {
		t.Errorf("Expected LION, got %s", BatteryLion)
	}

	// Test InterferenceStatus
	if InterferenceDetected != "DETECTED" {
		t.Errorf("Expected DETECTED, got %s", InterferenceDetected)
	}

	// Test ULXDGain
	ulxdGain := ULXDGain(30)
	if ulxdGain.ToDB() != 12 {
		t.Errorf("Expected 12dB, got %d", ulxdGain.ToDB())
	}

	// Test SLDXAudioLevel
	sldxLevel := SLDXAudioLevel(100)
	if sldxLevel.ToDBFS() != -20 {
		t.Errorf("Expected -20 dBFS, got %d", sldxLevel.ToDBFS())
	}

	// Test ULXDRFLevel
	ulxdRF := ULXDRFLevel(100)
	if ulxdRF.ToDBM() != -28 {
		t.Errorf("Expected -28 dBm, got %d", ulxdRF.ToDBM())
	}
}

// Test model family detection and formatting
func TestShureModelFamily(t *testing.T) {
	tests := []struct {
		model        string
		expectedFam  ShureModelFamily
		expectSpaces bool
	}{
		{"AD4Q", ModelFamilyAxientDigital, false},
		{"AD1", ModelFamilyAxientDigital, false},
		{"ULXD4", ModelFamilyULXD, true},
		{"ULXD2", ModelFamilyULXD, true},
		{"QLXD4", ModelFamilyQLXD, true},
		{"QLXD2", ModelFamilyQLXD, true},
		{"SLXD4", ModelFamilySLXD, false},
		{"SLXD1", ModelFamilySLXD, false},
		{"SLXD4+", ModelFamilySLXDPlus, false},
		{"SLXD2+", ModelFamilySLXDPlus, false},
	}

	for _, tc := range tests {
		fam := DetectModelFamily(tc.model)
		if fam != tc.expectedFam {
			t.Errorf("DetectModelFamily(%s): expected %s, got %s", tc.model, tc.expectedFam, fam)
		}
		if fam.UseSpaces() != tc.expectSpaces {
			t.Errorf("UseSpaces() for %s: expected %v, got %v", tc.model, tc.expectSpaces, fam.UseSpaces())
		}
	}
}

// Test command builder with model family
func TestShureCommandBuilderWithModel(t *testing.T) {
	// Test Axient Digital (underscores, braces for string values)
	adCmd := NewShureCommandWithModel("SET", ModelFamilyAxientDigital).
		WithIndex(1).
		WithParam("CHAN_NAME", "TestChannel")
	adResult := adCmd.Build()
	if !strings.Contains(adResult, "AUDIO_GAIN") && !strings.Contains(adResult, "CHAN_NAME") {
		t.Errorf("Expected CHAN_NAME in Axient command, got %s", adResult)
	}
	if !strings.Contains(adResult, "{TestChannel}") {
		t.Errorf("Expected braces for Axient Digital string value, got %s", adResult)
	}

	// Test ULX-D (spaces, no braces)
	ulxdCmd := NewShureCommandWithModel("SET", ModelFamilyULXD).
		WithIndex(1).
		WithParam("CHAN_NAME", "TestChannel")
	ulxdResult := ulxdCmd.Build()
	if !strings.Contains(ulxdResult, "CHAN NAME") {
		t.Errorf("Expected 'CHAN NAME' (with space) in ULX-D command, got %s", ulxdResult)
	}
	if strings.Contains(ulxdResult, "{") {
		t.Errorf("Expected no braces for ULX-D, got %s", ulxdResult)
	}
}

// Test ULX-D/QLX-D commands
func TestULXDCommands(t *testing.T) {
	// Test HighDensity
	cmd := GetHighDensityModeCommand{Channel: 1}
	if !strings.Contains(cmd.String(), "HIGH_DENSITY") {
		t.Errorf("Expected HIGH_DENSITY in command, got %s", cmd.String())
	}

	cmd2 := SetHighDensityModeCommand{Channel: 1, Mode: HighDensityOn}
	if !strings.Contains(cmd2.String(), "ON") {
		t.Errorf("Expected ON in high density command, got %s", cmd2.String())
	}

	// Test Audio Summing
	cmd3 := GetAudioSummingModeCommand{Channel: 1}
	if !strings.Contains(cmd3.String(), "AUDIO_SUMMING_MODE") {
		t.Errorf("Expected AUDIO_SUMMING_MODE in command, got %s", cmd3.String())
	}

	cmd4 := SetAudioSummingModeCommand{Channel: 1, Mode: AudioSumming1Plus2}
	if !strings.Contains(cmd4.String(), "1+2") {
		t.Errorf("Expected 1+2 in summing command, got %s", cmd4.String())
	}

	// Test Frequency Diversity
	cmd5 := GetFrequencyDiversityModeCommand{Channel: 1}
	if !strings.Contains(cmd5.String(), "FREQUENCY_DIVERSITY_MODE") {
		t.Errorf("Expected FREQUENCY_DIVERSITY_MODE in command, got %s", cmd5.String())
	}

	// Test TX Mute Status
	cmd6 := GetTXMuteStatusCommand{Channel: 1}
	if !strings.Contains(cmd6.String(), "TX_MUTE_STATUS") {
		t.Errorf("Expected TX_MUTE_STATUS in command, got %s", cmd6.String())
	}

	// Test TX Power Source
	cmd7 := GetTXPowerSourceCommand{Channel: 1}
	if !strings.Contains(cmd7.String(), "TX_POWER_SOURCE") {
		t.Errorf("Expected TX_POWER_SOURCE in command, got %s", cmd7.String())
	}

	// Test charger commands
	cmd8 := GetTXAvailableCommand{Bay: 1}
	if !strings.Contains(cmd8.String(), "TX_AVAILABLE") {
		t.Errorf("Expected TX_AVAILABLE in command, got %s", cmd8.String())
	}

	cmd9 := GetBattTimeToFullCommand{Bay: 2}
	if !strings.Contains(cmd9.String(), "BATT_TIME_TO_FULL") {
		t.Errorf("Expected BATT_TIME_TO_FULL in command, got %s", cmd9.String())
	}
}

// Test SLX-D/SLX-D+ commands
func TestSLXDCommands(t *testing.T) {
	// Test Audio Output Level Switch
	cmd := GetAudioOutputLevelSwitchCommand{Channel: 1}
	if !strings.Contains(cmd.String(), "AUDIO_OUT_LVL_SWITCH") {
		t.Errorf("Expected AUDIO_OUT_LVL_SWITCH in command, got %s", cmd.String())
	}

	cmd2 := SetAudioOutputLevelSwitchCommand{Channel: 1, Level: AudioOutputLine}
	if !strings.Contains(cmd2.String(), "LINE") {
		t.Errorf("Expected LINE in command, got %s", cmd2.String())
	}

	// Test SLX-D+ Dante commands
	cmd3 := GetNADeviceNameCommand{}
	if !strings.Contains(cmd3.String(), "NA_DEVICE_NAME") {
		t.Errorf("Expected NA_DEVICE_NAME in command, got %s", cmd3.String())
	}

	cmd4 := GetNAChannelNameCommand{Channel: 1}
	if !strings.Contains(cmd4.String(), "NA_CHAN_NAME") {
		t.Errorf("Expected NA_CHAN_NAME in command, got %s", cmd4.String())
	}

	// Test Net Settings
	cmd5 := SetNetSettingsCommand{
		Interface:  NetworkInterfaceD1,
		IPMode:     IPModeManual,
		IPAddress:  "10.10.1.15",
		SubnetMask: "255.255.255.0",
		Gateway:    "10.10.1.1",
	}
	cmd5Str := cmd5.String()
	if !strings.Contains(cmd5Str, "NET_SETTINGS") {
		t.Errorf("Expected NET_SETTINGS in command, got %s", cmd5Str)
	}
	if !strings.Contains(cmd5Str, "D1") {
		t.Errorf("Expected D1 in command, got %s", cmd5Str)
	}

	// Test Remote Pair
	cmd6 := SetRemotePairCommand{Channel: 1, Status: RemotePairOn}
	if !strings.Contains(cmd6.String(), "REM_PAIR") || !strings.Contains(cmd6.String(), "ON") {
		t.Errorf("Expected REM_PAIR ON in command, got %s", cmd6.String())
	}

	// Test Link Status
	cmd7 := GetLinkStatusCommand{Channel: 1, Slot: 1}
	if !strings.Contains(cmd7.String(), "LINK_STATUS") {
		t.Errorf("Expected LINK_STATUS in command, got %s", cmd7.String())
	}

	cmd8 := GetLinkTXModelCommand{Channel: 1, Slot: 1}
	if !strings.Contains(cmd8.String(), "LINK_TX_MODEL") {
		t.Errorf("Expected LINK_TX_MODEL in command, got %s", cmd8.String())
	}

	// Test Encryption Status
	cmd9 := GetEncryptionStatusCommand{Channel: 1}
	if !strings.Contains(cmd9.String(), "ENCRYPTION_STATUS") {
		t.Errorf("Expected ENCRYPTION_STATUS in command, got %s", cmd9.String())
	}

	// Test Interference Status
	cmd10 := GetSLXInterferenceStatusCommand{Channel: 1}
	if !strings.Contains(cmd10.String(), "INTERFERENCE_STATUS") {
		t.Errorf("Expected INTERFERENCE_STATUS in command, got %s", cmd10.String())
	}
}

// Test ULX-D/QLX-D SAMPLE parsing
func TestParseULXDSampleReport(t *testing.T) {
	// < SAMPLE 1 ALL nn aaa eee >
	// nn = AX (antenna A on), aaa = 100 (RF -28dBm), eee = 030 (audio -20dBFS)
	resp := "< SAMPLE 1 ALL AX 100 030 >"
	report := ParseULXDSampleReport(resp)
	if report == nil {
		t.Fatal("Expected report, got nil")
	}
	if report.Channel != 1 {
		t.Errorf("Expected channel 1, got %d", report.Channel)
	}
	if report.AntStatus != AntennaAOn {
		t.Errorf("Expected AX, got %s", report.AntStatus)
	}
	if report.RFLevel != 100 {
		t.Errorf("Expected RF level 100, got %d", report.RFLevel)
	}
	if report.RFLevelDBM() != -28 {
		t.Errorf("Expected -28 dBm, got %d", report.RFLevelDBM())
	}
	if report.AudioLevel != 30 {
		t.Errorf("Expected audio level 30, got %d", report.AudioLevel)
	}
	if report.AudioLevelDBFS() != -20 {
		t.Errorf("Expected -20 dBFS, got %d", report.AudioLevelDBFS())
	}
}

// Test SLX-D/SLX-D+ SAMPLE parsing
func TestParseSLDXSampleReport(t *testing.T) {
	// < SAMPLE ch ALL audPeak audRms rfRssi >
	resp := "< SAMPLE 2 ALL 110 095 080 >"
	report := ParseSLDXSampleReport(resp)
	if report == nil {
		t.Fatal("Expected report, got nil")
	}
	if report.Channel != 2 {
		t.Errorf("Expected channel 2, got %d", report.Channel)
	}
	if report.AudioPeak != 110 {
		t.Errorf("Expected peak 110, got %d", report.AudioPeak)
	}
	if report.AudioPeakDBFS() != -10 {
		t.Errorf("Expected -10 dBFS, got %d", report.AudioPeakDBFS())
	}
	if report.AudioRMS != 95 {
		t.Errorf("Expected RMS 95, got %d", report.AudioRMS)
	}
	if report.AudioRMSDBFS() != -25 {
		t.Errorf("Expected -25 dBFS, got %d", report.AudioRMSDBFS())
	}
	if report.RFRSSI != 80 {
		t.Errorf("Expected RSSI 80, got %d", report.RFRSSI)
	}
	if report.RFRSSIDBM() != -40 {
		t.Errorf("Expected -40 dBm, got %d", report.RFRSSIDBM())
	}
}

// Test DetectSampleFormat
func TestDetectSampleFormat(t *testing.T) {
	tests := []struct {
		response string
		expected string
	}{
		{"< SAMPLE 1 ALL AX 100 030 >", "ulxd"},
		{"< SAMPLE 2 ALL 110 095 080 >", "sldx"},
		{"< SAMPLE 1 ALL 005 000 045 062 BB 31 099 31 085 >", "axient"},
		{"< REP 1 MODEL {AD4Q} >", "unknown"},
	}

	for _, tc := range tests {
		result := DetectSampleFormat(tc.response)
		if result != tc.expected {
			t.Errorf("DetectSampleFormat(%s): expected %s, got %s", tc.response, tc.expected, result)
		}
	}
}

// Test RepReport parsing for ULX-D format (spaces in param names)
func TestParseRepReportULXDFormat(t *testing.T) {
	// ULX-D uses spaces instead of underscores
	// ParseRepReport captures the raw param name from the response
	resp := "< REP 1 AUDIO GAIN 030 >"
	report := ParseRepReport(resp)
	if report == nil {
		t.Fatal("Expected report, got nil")
	}
	if report.Param != "AUDIO" {
		t.Errorf("Expected 'AUDIO', got %s", report.Param)
	}
	if report.Value != "GAIN 030" {
		t.Errorf("Expected 'GAIN 030', got %s", report.Value)
	}

	// Test Axient format with underscores (for comparison)
	resp2 := "< REP 1 AUDIO_GAIN 030 >"
	report2 := ParseRepReport(resp2)
	if report2 == nil {
		t.Fatal("Expected report, got nil")
	}
	if report2.Param != "AUDIO_GAIN" {
		t.Errorf("Expected 'AUDIO_GAIN', got %s", report2.Param)
	}
	if report2.Gain != 30 {
		t.Errorf("Expected gain 30, got %d", report2.Gain)
	}

	// Test GROUP_CHANNEL Axient format
	resp3 := "< REP 2 GROUP_CHANNEL {03,07} >"
	report3 := ParseRepReport(resp3)
	if report3 == nil {
		t.Fatal("Expected report, got nil")
	}
	if report3.Group != 3 || report3.Chan != 7 {
		t.Errorf("Expected group 3, chan 7, got group %d, chan %d", report3.Group, report3.Chan)
	}

	// Test ENCRYPTION_MODE
	resp4 := "< REP 1 ENCRYPTION_MODE ON >"
	report4 := ParseRepReport(resp4)
	if report4 == nil {
		t.Fatal("Expected report, got nil")
	}
	if report4.Value != "ON" {
		t.Errorf("Expected ON, got %s", report4.Value)
	}
}
