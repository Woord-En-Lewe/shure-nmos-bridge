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
