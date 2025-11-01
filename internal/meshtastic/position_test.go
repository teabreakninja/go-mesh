package meshtastic

import (
	"testing"
)

// Test the enhanced PositionData structure and parsing
func TestEnhancedPositionData(t *testing.T) {
	// Test basic position data creation using protobuf generated struct
	latI := int32(377749000) // 37.7749 * 1e7
	lonI := int32(-1224194000) // -122.4194 * 1e7
	alt := int32(100)
	pos := &PositionData{
		LatitudeI:      &latI,
		LongitudeI:     &lonI,
		Altitude:       &alt,
		Time:           1640995200, // Jan 1, 2022
		LocationSource: Position_LOC_INTERNAL,
		AltitudeSource: Position_ALT_INTERNAL,
	}

	// Test that basic fields are accessible
	if pos.GetLatitudeDegrees() != 37.7749 {
		t.Errorf("Expected latitude 37.7749, got %f", pos.GetLatitudeDegrees())
	}
	if pos.GetLongitudeDegrees() != -122.4194 {
		t.Errorf("Expected longitude -122.4194, got %f", pos.GetLongitudeDegrees())
	}
	if pos.GetAltitude() != 100 {
		t.Errorf("Expected altitude 100, got %d", pos.GetAltitude())
	}

	// Test enhanced fields with pointer values
	timestamp := uint32(1640995200)
	pos.Timestamp = timestamp

	pdop := uint32(150) // 1.5 * 100
	pos.PDOP = pdop

	satellites := uint32(8)
	pos.SatsInView = satellites

	// Verify enhanced fields
	if pos.Timestamp != 1640995200 {
		t.Error("Timestamp field not set correctly")
	}
	if pos.PDOP != 150 {
		t.Error("PDOP field not set correctly")
	}
	if pos.SatsInView != 8 {
		t.Error("SatsInView field not set correctly")
	}

	// Test that optional fields work correctly
	if pos.GroundSpeed != nil {
		t.Error("GroundSpeed should be nil when not set")
	}
}

// Test LocationSource and AltitudeSource enums
func TestLocationAndAltitudeSources(t *testing.T) {
	// Test LocationSource enum values using protobuf generated enums
	if Position_LOC_UNSET != 0 {
		t.Errorf("Expected Position_LOC_UNSET to be 0, got %d", Position_LOC_UNSET)
	}
	if Position_LOC_INTERNAL != 2 {
		t.Errorf("Expected Position_LOC_INTERNAL to be 2, got %d", Position_LOC_INTERNAL)
	}
	if Position_LOC_MANUAL != 1 {
		t.Errorf("Expected Position_LOC_MANUAL to be 1, got %d", Position_LOC_MANUAL)
	}

	// Test AltitudeSource enum values using protobuf generated enums
	if Position_ALT_UNSET != 0 {
		t.Errorf("Expected Position_ALT_UNSET to be 0, got %d", Position_ALT_UNSET)
	}
	if Position_ALT_INTERNAL != 2 {
		t.Errorf("Expected Position_ALT_INTERNAL to be 2, got %d", Position_ALT_INTERNAL)
	}
	if Position_ALT_BAROMETRIC != 4 {
		t.Errorf("Expected Position_ALT_BAROMETRIC to be 4, got %d", Position_ALT_BAROMETRIC)
	}
}

// Test HardwareModel enum expansion
func TestHardwareModelEnum(t *testing.T) {
	// Test basic hardware models using protobuf generated enums
	if HardwareModel_UNSET != 0 {
		t.Errorf("Expected HardwareModel_UNSET to be 0, got %d", HardwareModel_UNSET)
	}
	if HardwareModel_TLORA_V2 != 1 {
		t.Errorf("Expected HardwareModel_TLORA_V2 to be 1, got %d", HardwareModel_TLORA_V2)
	}
	if HardwareModel_TBEAM != 4 {
		t.Errorf("Expected HardwareModel_TBEAM to be 4, got %d", HardwareModel_TBEAM)
	}

	// Test hardware model name function
	name := GetHardwareModelName(HardwareModel_TBEAM)
	if name != "TBEAM" {
		t.Errorf("Expected 'TBEAM', got '%s'", name)
	}

	name = GetHardwareModelName(HardwareModel_RAK4631)
	if name != "RAK4631" {
		t.Errorf("Expected 'RAK4631', got '%s'", name)
	}

	// Test some new hardware models
	if HardwareModel_HELTEC_V3 != 43 {
		t.Errorf("Expected HardwareModel_HELTEC_V3 to be 43, got %d", HardwareModel_HELTEC_V3)
	}

	name = GetHardwareModelName(HardwareModel_HELTEC_V3)
	if name != "HELTEC_V3" {
		t.Errorf("Expected 'HELTEC_V3', got '%s'", name)
	}
}

// Test position message parsing with mock data
func TestPositionMessageParsing(t *testing.T) {
	// Create mock position data with basic fields
	// This is a simplified test - in practice, you'd need proper protobuf encoding
	mockData := []byte{
		0x0D, 0x00, 0x00, 0x00, 0x00, // Field 1: latitude_i (fixed32)
		0x15, 0x00, 0x00, 0x00, 0x00, // Field 2: longitude_i (fixed32)
		0x18, 0x64,                   // Field 3: altitude (varint 100)
		0x25, 0x00, 0x00, 0x00, 0x00, // Field 4: time (fixed32)
	}

	pos := parsePositionMessage(mockData)
	if pos == nil {
		t.Error("parsePositionMessage returned nil")
		return
	}

	// The exact values depend on the binary encoding, but we should get a valid position
	t.Logf("Parsed position: lat=%f, lon=%f, alt=%d", pos.Latitude, pos.Longitude, pos.Altitude)
}
