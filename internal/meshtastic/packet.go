package meshtastic

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"go-mesh/pb"
	"google.golang.org/protobuf/proto"
)

// PacketTypeStats tracks how many packets of each type we've seen
type PacketTypeStats struct {
	mu     sync.RWMutex
	counts map[PacketType]int
	total  int
}

var globalPacketStats = &PacketTypeStats{
	counts: make(map[PacketType]int),
}

// IncrementPacketType increments the count for a packet type
func (s *PacketTypeStats) IncrementPacketType(packetType PacketType) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[packetType]++
	s.total++
}

// GetStats returns a copy of current statistics
func (s *PacketTypeStats) GetStats() map[PacketType]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := make(map[PacketType]int)
	for k, v := range s.counts {
		stats[k] = v
	}
	return stats
}

// GetTotal returns total packets processed
func (s *PacketTypeStats) GetTotal() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.total
}

// GetStatsString returns formatted statistics string
func (s *PacketTypeStats) GetStatsString() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.total == 0 {
		return "No packets processed"
	}
	
	result := fmt.Sprintf("Total: %d packets\n", s.total)
	for packetType, count := range s.counts {
		percentage := float64(count) / float64(s.total) * 100
		typeName := PacketTypeNames[packetType]
		if typeName == "" {
			typeName = fmt.Sprintf("TYPE_%d", int(packetType))
		}
		result += fmt.Sprintf("  %s: %d (%.1f%%)\n", typeName, count, percentage)
	}
	return result
}

// GetGlobalPacketStats returns the global packet statistics
func GetGlobalPacketStats() *PacketTypeStats {
	return globalPacketStats
}

// Type aliases for protobuf generated structs
type (
	Position            = pb.Position
	HardwareModel       = pb.HardwareModel
	DeviceMetrics       = pb.DeviceMetrics
	EnvironmentMetrics  = pb.EnvironmentMetrics
	AirQualityMetrics   = pb.AirQualityMetrics
	PowerMetrics        = pb.PowerMetrics
	Telemetry          = pb.Telemetry
)

// UserData represents decoded user information (NODE_INFO packets)
// This matches the User message from mesh.proto
type UserData struct {
	ID             string        `json:"id"`
	LongName       string        `json:"long_name"`
	ShortName      string        `json:"short_name"`
	MacAddr        []byte        `json:"mac_addr,omitempty"`
	HwModel        HardwareModel `json:"hw_model"`
	IsLicensed     bool          `json:"is_licensed,omitempty"`
	Role           uint32        `json:"role,omitempty"`
	PublicKey      []byte        `json:"public_key,omitempty"`
	IsUnmessagable *bool         `json:"is_unmessagable,omitempty"`
}

// PacketType represents the type of Meshtastic packet
type PacketType uint32

const (
	PacketTypeUnknown PacketType = iota
	PacketTypePosition
	PacketTypeText
	PacketTypeTelemetry
	PacketTypeNodeInfo
	PacketTypeRouting
	PacketTypeAdmin
	PacketTypeRangeTest
	PacketTypeNeighborInfo
	PacketTypeDetectionSensor
	PacketTypeRemoteHardware
	PacketTypeReplyApp
	PacketTypeIpTunnelApp
	PacketTypeSerialApp
	PacketTypeStoreForwardApp
	PacketTypeRangeTestApp
	PacketTypeTelemetryApp
	PacketTypeZpsApp
	PacketTypeSimulatorApp
	PacketTypeTracerouteApp
)

var PacketTypeNames = map[PacketType]string{
	PacketTypeUnknown:             "UNKNOWN",
	PacketTypePosition:            "POSITION",
	PacketTypeText:                "TEXT",
	PacketTypeTelemetry:           "TELEMETRY",
	PacketTypeNodeInfo:            "NODE_INFO",
	PacketTypeRouting:             "ROUTING",
	PacketTypeAdmin:               "ADMIN",
	PacketTypeRangeTest:           "RANGE_TEST",
	PacketTypeNeighborInfo:        "NEIGHBOR_INFO",
	PacketTypeDetectionSensor:     "DETECTION_SENSOR",
	PacketTypeRemoteHardware:      "REMOTE_HARDWARE",
	PacketTypeReplyApp:            "REPLY_APP",
	PacketTypeIpTunnelApp:         "IP_TUNNEL_APP",
	PacketTypeSerialApp:           "SERIAL_APP",
	PacketTypeStoreForwardApp:     "STORE_FORWARD_APP",
	PacketTypeRangeTestApp:        "RANGE_TEST_APP",
	PacketTypeTelemetryApp:        "TELEMETRY_APP",
	PacketTypeZpsApp:              "ZPS_APP",
	PacketTypeSimulatorApp:        "SIMULATOR_APP",
	PacketTypeTracerouteApp:       "TRACEROUTE_APP",
}

// PortNum to PacketType mapping based on Meshtastic portnums
var PortNumToPacketType = map[uint32]PacketType{
	0:   PacketTypeUnknown,          // UNKNOWN_APP
	1:   PacketTypeText,             // TEXT_MESSAGE_APP
	2:   PacketTypeRemoteHardware,   // REMOTE_HARDWARE_APP
	3:   PacketTypePosition,         // POSITION_APP
	4:   PacketTypeNodeInfo,         // NODEINFO_APP
	5:   PacketTypeRouting,          // ROUTING_APP
	6:   PacketTypeAdmin,            // ADMIN_APP
	7:   PacketTypeText,             // TEXT_MESSAGE_COMPRESSED_APP
	8:   PacketTypePosition,         // WAYPOINT_APP (similar to position)
	9:   PacketTypeUnknown,          // AUDIO_APP
	10:  PacketTypeDetectionSensor,  // DETECTION_SENSOR_APP
	11:  PacketTypeText,             // ALERT_APP (similar to text)
	12:  PacketTypeUnknown,          // KEY_VERIFICATION_APP
	32:  PacketTypeReplyApp,         // REPLY_APP
	33:  PacketTypeIpTunnelApp,      // IP_TUNNEL_APP
	34:  PacketTypeUnknown,          // PAXCOUNTER_APP
	64:  PacketTypeSerialApp,        // SERIAL_APP
	65:  PacketTypeStoreForwardApp,  // STORE_FORWARD_APP
	66:  PacketTypeRangeTest,        // RANGE_TEST_APP
	67:  PacketTypeTelemetry,        // TELEMETRY_APP
	68:  PacketTypeZpsApp,           // ZPS_APP
	69:  PacketTypeSimulatorApp,     // SIMULATOR_APP
	70:  PacketTypeTracerouteApp,    // TRACEROUTE_APP
	71:  PacketTypeNeighborInfo,     // NEIGHBORINFO_APP
	224: PacketTypeReplyApp,         // ATAK_PLUGIN (private range)
	256: PacketTypeReplyApp,         // Private apps
}

// Packet represents a decoded Meshtastic packet
type Packet struct {
	ID            uint32        `json:"id"`
	From          uint32        `json:"from"`
	To            uint32        `json:"to"`
	Type          PacketType    `json:"type"`
	Channel       uint8         `json:"channel"`
	HopCount      uint8         `json:"hop_count"`
	HopLimit      uint8         `json:"hop_limit"`
	WantAck       bool          `json:"want_ack"`
	Priority      uint8         `json:"priority"`
	RxTime        time.Time     `json:"rx_time"`
	RxSNR         float32       `json:"rx_snr"`
	RxRSSI        int32         `json:"rx_rssi"`
	Payload       []byte        `json:"payload"`
	DecodedData   interface{}   `json:"decoded_data,omitempty"`
	Raw           []byte        `json:"raw"`
}

// PositionData is an alias for the protobuf generated Position struct
type PositionData = Position

// Helper functions to convert Position to latitude/longitude in degrees
func GetLatitudeDegrees(p *Position) float64 {
	if p.LatitudeI != nil {
		return float64(*p.LatitudeI) / 1e7
	}
	return 0
}

func GetLongitudeDegrees(p *Position) float64 {
	if p.LongitudeI != nil {
		return float64(*p.LongitudeI) / 1e7
	}
	return 0
}

// TextData represents decoded text message with enhanced categorization
type TextData struct {
	Text     string            `json:"text"`
	Category string            `json:"category,omitempty"` // e.g., "device_info", "config", "nodedb"
	Details  map[string]string `json:"details,omitempty"`  // Extracted key-value pairs
}

// NewTextData creates a TextData with automatic categorization
func NewTextData(text string) *TextData {
	td := &TextData{
		Text:    text,
		Details: make(map[string]string),
	}
	
	// Categorize the text and extract details
	td.categorizeAndExtractDetails()
	
	return td
}

// categorizeAndExtractDetails analyzes text to categorize and extract structured data
func (td *TextData) categorizeAndExtractDetails() {
	lowerText := strings.ToLower(td.Text)
	
	// Device and firmware information
	if strings.Contains(lowerText, "firmware") || strings.Contains(lowerText, "version") {
		td.Category = "device_info"
		td.extractKeyValuePairs([]string{"firmware", "version", "build", "hw", "hardware"})
	} else if strings.Contains(lowerText, "channel") && (strings.Contains(lowerText, "settings") || strings.Contains(lowerText, "config")) {
		td.Category = "channel_config"
		td.extractKeyValuePairs([]string{"channel", "frequency", "name", "psk", "bw", "sf", "cr"})
	} else if strings.Contains(lowerText, "preferences") || strings.Contains(lowerText, "config") {
		td.Category = "config"
		td.extractKeyValuePairs([]string{"region", "modem", "power", "bandwidth", "spread"})
	} else if strings.Contains(lowerText, "owner") || strings.Contains(lowerText, "user") || strings.Contains(lowerText, "node id") {
		td.Category = "node_info"
		td.extractKeyValuePairs([]string{"owner", "user", "name", "id", "short", "long"})
	} else if strings.Contains(lowerText, "device info") || strings.Contains(lowerText, "my info") {
		td.Category = "device_status"
		td.extractKeyValuePairs([]string{"battery", "voltage", "uptime", "heap", "nodes"})
	} else {
		td.Category = "general"
	}
}

// extractKeyValuePairs looks for key-value patterns in the text
func (td *TextData) extractKeyValuePairs(keys []string) {
	for _, key := range keys {
		// Try various patterns: "key: value", "key=value", "key value"
		patterns := []string{
			fmt.Sprintf(`(?i)%s\s*[:=]\s*([^\n,;]+)`, key),
			fmt.Sprintf(`(?i)%s\s+([^\n,;\s]+)`, key),
		}
		
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindStringSubmatch(td.Text)
			if len(matches) > 1 {
				value := strings.TrimSpace(matches[1])
				if value != "" {
					td.Details[key] = value
					break
				}
			}
		}
	}
}

// TelemetryData is an alias for the protobuf generated Telemetry struct
type TelemetryData = Telemetry

// GetHardwareModelName returns a human-readable name for the hardware model
func GetHardwareModelName(model HardwareModel) string {
	return model.String()
}


// NodeInfo represents decoded node information
type NodeInfo struct {
	ID        string        `json:"id"`
	LongName  string        `json:"long_name"`
	ShortName string        `json:"short_name"`
	MacAddr   []byte        `json:"mac_addr"`
	HwModel   HardwareModel `json:"hw_model"`
	Role      uint32        `json:"role"`
}

// GetHardwareModelName returns the hardware model name for the node
func (n *NodeInfo) GetHardwareModelName() string {
	return GetHardwareModelName(n.HwModel)
}

// User represents user information from mesh.proto
type User struct {
	ID             string `json:"id"`
	LongName       string `json:"long_name"`
	ShortName      string `json:"short_name"`
	MacAddr        []byte `json:"mac_addr"`
	HwModel        uint32 `json:"hw_model"`
	IsLicensed     bool   `json:"is_licensed,omitempty"`
	Role           uint32 `json:"role,omitempty"`
	PublicKey      []byte `json:"public_key,omitempty"`
}

// RouteInfo represents routing information
type RouteInfo struct {
	Route []uint32 `json:"route"`
	SNRs  []int8   `json:"snrs"`
}

// RemoteHardwareMessage represents decoded remote hardware information
type RemoteHardwareMessage struct {
	Type      RemoteHardwareType `json:"type"`       // What type of hardware message
	GpioMask  uint64             `json:"gpio_mask"`  // Which GPIOs are affected
	GpioValue uint64             `json:"gpio_value"` // GPIO signal levels
}

// RemoteHardwareType represents the type of remote hardware operation
type RemoteHardwareType uint32

const (
	RemoteHardwareUnset        RemoteHardwareType = 0 // Unset/unused
	RemoteHardwareWriteGpios   RemoteHardwareType = 1 // Set GPIO pins based on gpio_mask/gpio_value
	RemoteHardwareWatchGpios   RemoteHardwareType = 2 // Watch GPIO pins for changes
	RemoteHardwareGpiosChanged RemoteHardwareType = 3 // GPIO pins have changed values
	RemoteHardwareReadGpios    RemoteHardwareType = 4 // Read GPIO pins
	RemoteHardwareReadReply    RemoteHardwareType = 5 // Reply to READ GPIO request
)

// RemoteHardwareTypeNames maps hardware types to human-readable names
var RemoteHardwareTypeNames = map[RemoteHardwareType]string{
	RemoteHardwareUnset:        "UNSET",
	RemoteHardwareWriteGpios:   "WRITE_GPIOS",
	RemoteHardwareWatchGpios:   "WATCH_GPIOS",
	RemoteHardwareGpiosChanged: "GPIOS_CHANGED",
	RemoteHardwareReadGpios:    "READ_GPIOS",
	RemoteHardwareReadReply:    "READ_GPIOS_REPLY",
}

// GetTypeName returns human-readable name for the remote hardware type
func (r RemoteHardwareType) GetTypeName() string {
	if name, ok := RemoteHardwareTypeNames[r]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", r)
}

// GetAffectedGpios returns a list of GPIO pin numbers that are affected by gpio_mask
func (r *RemoteHardwareMessage) GetAffectedGpios() []int {
	var gpios []int
	for i := 0; i < 64; i++ {
		if (r.GpioMask & (1 << i)) != 0 {
			gpios = append(gpios, i)
		}
	}
	return gpios
}

// GetGpioStates returns a map of GPIO pin numbers to their current state (true=HIGH, false=LOW)
func (r *RemoteHardwareMessage) GetGpioStates() map[int]bool {
	states := make(map[int]bool)
	for i := 0; i < 64; i++ {
		if (r.GpioMask & (1 << i)) != 0 {
			states[i] = (r.GpioValue & (1 << i)) != 0
		}
	}
	return states
}

// FormatGpioInfo returns a human-readable string describing the GPIO operations
func (r *RemoteHardwareMessage) FormatGpioInfo() string {
	gpios := r.GetAffectedGpios()
	if len(gpios) == 0 {
		return "No GPIOs"
	}

	switch r.Type {
	case RemoteHardwareWriteGpios:
		states := r.GetGpioStates()
		var parts []string
		for _, pin := range gpios {
			state := "LOW"
			if states[pin] {
				state = "HIGH"
			}
			parts = append(parts, fmt.Sprintf("GPIO%d=%s", pin, state))
		}
		return fmt.Sprintf("Write: %s", strings.Join(parts, ", "))

	case RemoteHardwareWatchGpios:
		var pinStrs []string
		for _, pin := range gpios {
			pinStrs = append(pinStrs, fmt.Sprintf("GPIO%d", pin))
		}
		return fmt.Sprintf("Watch: %s", strings.Join(pinStrs, ", "))

	case RemoteHardwareGpiosChanged:
		states := r.GetGpioStates()
		var parts []string
		for _, pin := range gpios {
			state := "LOW"
			if states[pin] {
				state = "HIGH"
			}
			parts = append(parts, fmt.Sprintf("GPIO%d=%s", pin, state))
		}
		return fmt.Sprintf("Changed: %s", strings.Join(parts, ", "))

	case RemoteHardwareReadGpios:
		var pinStrs []string
		for _, pin := range gpios {
			pinStrs = append(pinStrs, fmt.Sprintf("GPIO%d", pin))
		}
		return fmt.Sprintf("Read: %s", strings.Join(pinStrs, ", "))

	case RemoteHardwareReadReply:
		states := r.GetGpioStates()
		var parts []string
		for _, pin := range gpios {
			state := "LOW"
			if states[pin] {
				state = "HIGH"
			}
			parts = append(parts, fmt.Sprintf("GPIO%d=%s", pin, state))
		}
		return fmt.Sprintf("Reply: %s", strings.Join(parts, ", "))

	default:
		var pinStrs []string
		for _, pin := range gpios {
			pinStrs = append(pinStrs, fmt.Sprintf("GPIO%d", pin))
		}
		return fmt.Sprintf("GPIOs: %s", strings.Join(pinStrs, ", "))
	}
}

// GetTypeName returns the human-readable name for the packet type
func (p *Packet) GetTypeName() string {
	if name, ok := PacketTypeNames[p.Type]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", p.Type)
}

// GetFromHex returns the sender address as hex string
func (p *Packet) GetFromHex() string {
	return fmt.Sprintf("!%08x", p.From)
}

// GetToHex returns the destination address as hex string
func (p *Packet) GetToHex() string {
	return fmt.Sprintf("!%08x", p.To)
}

// GetFromName returns the friendly name for the sender using the provided NodeDB
func (p *Packet) GetFromName(nodeDB *NodeDB) string {
	if nodeDB == nil {
		return p.GetFromHex()
	}
	return nodeDB.GetNodeName(p.From)
}

// GetToName returns the friendly name for the destination using the provided NodeDB
func (p *Packet) GetToName(nodeDB *NodeDB) string {
	if nodeDB == nil {
		return p.GetToHex()
	}
	if p.IsToAll() {
		return "ALL"
	}
	return nodeDB.GetNodeName(p.To)
}

// GetFromShortName returns the short friendly name for the sender using the provided NodeDB
func (p *Packet) GetFromShortName(nodeDB *NodeDB) string {
	if nodeDB == nil {
		return p.GetFromHex()
	}
	return nodeDB.GetNodeShortName(p.From)
}

// GetToShortName returns the short friendly name for the destination using the provided NodeDB
func (p *Packet) GetToShortName(nodeDB *NodeDB) string {
	if nodeDB == nil {
		return p.GetToHex()
	}
	if p.IsToAll() {
		return "ALL"
	}
	return nodeDB.GetNodeShortName(p.To)
}

// IsToAll returns true if the packet is a broadcast
func (p *Packet) IsToAll() bool {
	return p.To == 0xFFFFFFFF
}

// GetSignalStrength returns a human-readable signal strength
func (p *Packet) GetSignalStrength() string {
	if p.RxRSSI == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%d dBm (SNR: %.1f)", p.RxRSSI, p.RxSNR)
}

// GetHopInfo returns hop information as a string
func (p *Packet) GetHopInfo() string {
	// Handle cases where parsing might be invalid (simplified parser)
	if p.HopCount > 10 || p.HopLimit > 10 || p.HopLimit == 0 {
		return "?/?"
	}
	return fmt.Sprintf("%d/%d", p.HopCount, p.HopLimit)
}

// ToJSON converts the packet to JSON for display/logging
func (p *Packet) ToJSON() (string, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseRawPacket attempts to parse a raw packet from serial data
func ParseRawPacket(data []byte) (*Packet, error) {
	if len(data) < 16 { // Minimum packet size
		return nil, fmt.Errorf("packet too short: %d bytes", len(data))
	}

	packet := &Packet{
		RxTime: time.Now(),
		Raw:    data,
	}

	// This is a simplified parser - in a real implementation,
	// you'd use the actual Meshtastic protobuf definitions
	if len(data) >= 4 {
		packet.ID = binary.LittleEndian.Uint32(data[0:4])
	}
	if len(data) >= 8 {
		packet.From = binary.LittleEndian.Uint32(data[4:8])
	}
	if len(data) >= 12 {
		packet.To = binary.LittleEndian.Uint32(data[8:12])
	}
	if len(data) >= 16 {
		flags := binary.LittleEndian.Uint32(data[12:16])
		packet.Channel = uint8((flags >> 0) & 0xFF)
		packet.HopCount = uint8((flags >> 8) & 0xFF)
		packet.HopLimit = uint8((flags >> 16) & 0xFF)
		packet.Priority = uint8((flags >> 24) & 0xFF)
	}

	// Extract payload
	if len(data) > 16 {
		packet.Payload = data[16:]
		packet.Type = inferPacketType(packet.Payload)
		packet.DecodedData = decodePayload(packet.Type, packet.Payload)
		
		// Track statistics
		globalPacketStats.IncrementPacketType(packet.Type)
	}

	return packet, nil
}

// inferPacketType attempts to determine packet type from payload
func inferPacketType(payload []byte) PacketType {
	if len(payload) == 0 {
		return PacketTypeUnknown
	}

	// Try to detect protobuf message types by attempting to unmarshal with each type
	// This is more reliable than guessing from field tags
	
	// Try Position first (common and distinctive)
	pos := &Position{}
	if proto.Unmarshal(payload, pos) == nil {
		if pos.LatitudeI != nil || pos.LongitudeI != nil || pos.Altitude != nil {
			return PacketTypePosition
		}
	}
	
	// Try Telemetry
	tel := &Telemetry{}
	if proto.Unmarshal(payload, tel) == nil {
		if tel.DeviceMetrics != nil || tel.EnvironmentMetrics != nil || tel.AirQualityMetrics != nil {
			return PacketTypeTelemetry
		}
	}
	
	// Try User message (NODE_INFO)
	user := parseUserMessage(payload)
	if user != nil && (user.ID != "" || user.LongName != "" || user.ShortName != "") {
		return PacketTypeNodeInfo
	}
	
	// Enhanced protobuf field detection as fallback
	// Check for protobuf field tags (varint encoded)
	for i := 0; i < len(payload) && i < 10; i++ {
		b := payload[i]
		
		// Position packet indicators (protobuf field 1 for latitude)
		if b == 0x0D { // Fixed32 field 1 (latitude_i)
			return PacketTypePosition
		}
		
		// Text message indicators (protobuf field 1 for text)
		if b == 0x0A { // Length-delimited field 1 (text)
			return PacketTypeText
		}
		
		// Telemetry indicators
		if b == 0x08 || b == 0x10 || b == 0x18 { // Various varint fields common in telemetry
			return PacketTypeTelemetry
		}
		
		// Node info indicators
		if b == 0x12 { // Length-delimited field 2 (could be user info)
			return PacketTypeNodeInfo
		}
	}

	// Enhanced content analysis
	// Look for JSON structures (WiFi API responses)
	if len(payload) > 10 && payload[0] == '{' {
		return PacketTypeText
	}
	
	// Look for typical telemetry patterns (battery, voltage, etc.)
	if containsTelemetryKeywords(payload) {
		return PacketTypeTelemetry
	}
	
	// Look for position data patterns (coordinates)
	if containsPositionKeywords(payload) {
		return PacketTypePosition
	}

	// Check for printable text content
	printableCount := 0
	for _, b := range payload {
		if (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t' {
			printableCount++
		}
	}
	
	// If mostly printable, treat as text
	if len(payload) > 0 && float64(printableCount)/float64(len(payload)) > 0.7 {
		return PacketTypeText
	}

	return PacketTypeUnknown
}

// decodePayload attempts to decode the payload based on packet type
func decodePayload(packetType PacketType, payload []byte) interface{} {
	switch packetType {
	case PacketTypeText:
		// Remove null terminators and return as string
		end := len(payload)
		for i, b := range payload {
			if b == 0 {
				end = i
				break
			}
		}
		return &TextData{Text: string(payload[:end])}

	case PacketTypePosition:
		return parsePositionMessage(payload)

	case PacketTypeTelemetry:
		return parseTelemetryMessage(payload)

	case PacketTypeNodeInfo:
		return parseUserMessage(payload)

	case PacketTypeRemoteHardware:
		return parseRemoteHardwareMessage(payload)
	}

	return nil
}

// containsTelemetryKeywords checks if payload contains telemetry-related keywords
func containsTelemetryKeywords(payload []byte) bool {
	text := string(payload)
	keywords := []string{"battery", "voltage", "current", "temperature", "humidity", "pressure", "telemetry", "sensor"}
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(text), keyword) {
			return true
		}
	}
	return false
}

// parsePositionMessage parses a Position protobuf message using protobuf unmarshaling
func parsePositionMessage(data []byte) *PositionData {
	pos := &Position{}
	if err := proto.Unmarshal(data, pos); err != nil {
		return nil
	}
	return pos
}

// Helper functions for position parsing
func readVarint(data []byte, offset int) (uint64, int) {
	var result uint64
	var shift uint
	current := offset

	for current < len(data) {
		b := data[current]
		current++

		result |= uint64(b&0x7F) << shift

		if b&0x80 == 0 {
			return result, current
		}

		shift += 7
		if shift >= 64 {
			return 0, -1
		}
	}

	return 0, -1
}

func skipPositionField(data []byte, offset int, wireType int) int {
	switch wireType {
	case 0: // Varint
		_, newOffset := readVarint(data, offset)
		return newOffset
	case 1: // Fixed64
		return offset + 8
	case 2: // Length-delimited
		length, newOffset := readVarint(data, offset)
		if newOffset == -1 {
			return -1
		}
		return newOffset + int(length)
	case 5: // Fixed32
		return offset + 4
	default:
		return -1
	}
}

// parseTelemetryMessage parses a Telemetry protobuf message using protobuf unmarshaling
func parseTelemetryMessage(data []byte) *TelemetryData {
	tel := &Telemetry{}
	if err := proto.Unmarshal(data, tel); err != nil {
		return nil
	}
	return tel
}

// parseUserMessage parses a User protobuf message (NODE_INFO packets) using protobuf unmarshaling
func parseUserMessage(data []byte) *UserData {
	if len(data) < 2 {
		return nil
	}
	
	// For now, use manual parsing since pb types are complex
	// We'll parse the basic fields manually
	
	
	// Manually parse the key fields we need
	userData := &UserData{}
	offset := 0
	
	for offset < len(data) {
		if offset >= len(data) {
			break
		}
		
		tag := data[offset]
		fieldNumber := tag >> 3
		wireType := tag & 0x07
		offset++
		
		switch fieldNumber {
		case 1: // id (string)
			if wireType == 2 { // Length-delimited
				length, newOffset := readVarint(data, offset)
				if newOffset != -1 && newOffset+int(length) <= len(data) {
					userData.ID = string(data[newOffset:newOffset+int(length)])
					offset = newOffset + int(length)
				} else {
					offset = len(data)
				}
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}
			
		case 2: // long_name (string)
			if wireType == 2 { // Length-delimited
				length, newOffset := readVarint(data, offset)
				if newOffset != -1 && newOffset+int(length) <= len(data) {
					userData.LongName = string(data[newOffset:newOffset+int(length)])
					offset = newOffset + int(length)
				} else {
					offset = len(data)
				}
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}
			
		case 3: // short_name (string)
			if wireType == 2 { // Length-delimited
				length, newOffset := readVarint(data, offset)
				if newOffset != -1 && newOffset+int(length) <= len(data) {
					userData.ShortName = string(data[newOffset:newOffset+int(length)])
					offset = newOffset + int(length)
				} else {
					offset = len(data)
				}
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}
			
		default:
			// Skip other fields for now
			offset = skipPositionField(data, offset, int(wireType))
		}
		
		if offset == -1 {
			break
		}
	}
	
	// Debug logging removed to avoid interfering with TUI
	
	return userData
}

// parseDeviceMetrics parses DeviceMetrics protobuf message
func parseDeviceMetrics(data []byte) *DeviceMetrics {
	if len(data) < 2 {
		return nil
	}

	metrics := &DeviceMetrics{}
	offset := 0

	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		tag := data[offset]
		fieldNumber := tag >> 3
		wireType := tag & 0x07
		offset++

		switch fieldNumber {
		case 1: // battery_level
			if wireType == 0 { // Varint
				value, newOffset := readVarint(data, offset)
				if newOffset != -1 {
					metrics.BatteryLevel = uint32(value)
				}
				offset = newOffset
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}

		case 2: // voltage
			if wireType == 5 { // Fixed32 (float)
				if offset+4 <= len(data) {
					bits := binary.LittleEndian.Uint32(data[offset:offset+4])
					metrics.Voltage = math.Float32frombits(bits)
					offset += 4
				} else {
					offset = len(data)
				}
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}

		case 3: // channel_utilization
			if wireType == 5 { // Fixed32 (float)
				if offset+4 <= len(data) {
					bits := binary.LittleEndian.Uint32(data[offset:offset+4])
					metrics.ChannelUtilization = math.Float32frombits(bits)
					offset += 4
				} else {
					offset = len(data)
				}
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}

		case 4: // air_util_tx
			if wireType == 5 { // Fixed32 (float)
				if offset+4 <= len(data) {
					bits := binary.LittleEndian.Uint32(data[offset:offset+4])
					metrics.AirUtilTx = math.Float32frombits(bits)
					offset += 4
				} else {
					offset = len(data)
				}
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}

		case 5: // uptime_seconds
			if wireType == 0 { // Varint
				value, newOffset := readVarint(data, offset)
				if newOffset != -1 {
					metrics.UptimeSeconds = uint32(value)
				}
				offset = newOffset
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}

		default:
			// Skip unknown fields
			offset = skipPositionField(data, offset, int(wireType))
		}

		if offset == -1 {
			break
		}
	}

	return metrics
}

// parseEnvironmentMetrics parses EnvironmentMetrics protobuf message
func parseEnvironmentMetrics(data []byte) *EnvironmentMetrics {
	if len(data) < 2 {
		return nil
	}

	metrics := &EnvironmentMetrics{}
	offset := 0

	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		tag := data[offset]
		fieldNumber := tag >> 3
		wireType := tag & 0x07
		offset++

		switch fieldNumber {
		case 1: // temperature
			if wireType == 5 { // Fixed32 (float)
				if offset+4 <= len(data) {
					bits := binary.LittleEndian.Uint32(data[offset:offset+4])
					metrics.Temperature = math.Float32frombits(bits)
					offset += 4
				} else {
					offset = len(data)
				}
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}

		case 2: // relative_humidity
			if wireType == 5 { // Fixed32 (float)
				if offset+4 <= len(data) {
					bits := binary.LittleEndian.Uint32(data[offset:offset+4])
					metrics.RelativeHumidity = math.Float32frombits(bits)
					offset += 4
				} else {
					offset = len(data)
				}
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}

		case 3: // barometric_pressure
			if wireType == 5 { // Fixed32 (float)
				if offset+4 <= len(data) {
					bits := binary.LittleEndian.Uint32(data[offset:offset+4])
					metrics.BarometricPressure = math.Float32frombits(bits)
					offset += 4
				} else {
					offset = len(data)
				}
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}

		case 4: // gas_resistance
			if wireType == 5 { // Fixed32 (float)
				if offset+4 <= len(data) {
					bits := binary.LittleEndian.Uint32(data[offset:offset+4])
					metrics.GasResistance = math.Float32frombits(bits)
					offset += 4
				} else {
					offset = len(data)
				}
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}

		// Add cases for other environment fields (voltage, current, iaq, distance, lux values, wind, weight)
		// For brevity, I'll implement key fields. Full implementation would include all fields.

		default:
			// Skip unknown fields
			offset = skipPositionField(data, offset, int(wireType))
		}

		if offset == -1 {
			break
		}
	}

	return metrics
}

// parseAirQualityMetrics parses AirQualityMetrics protobuf message
func parseAirQualityMetrics(data []byte) *AirQualityMetrics {
	// Implementation would parse PM values and particle counts
	// For brevity, returning a basic structure
	return &AirQualityMetrics{}
}

// parsePowerMetrics parses PowerMetrics protobuf message
func parsePowerMetrics(data []byte) *PowerMetrics {
	// Implementation would parse power channel data
	// For brevity, returning a basic structure
	return &PowerMetrics{}
}

// parseRemoteHardwareMessage parses a RemoteHardware protobuf message
func parseRemoteHardwareMessage(data []byte) *RemoteHardwareMessage {
	if len(data) < 2 {
		return nil
	}

	hw := &RemoteHardwareMessage{
		Type: RemoteHardwareUnset,
	}
	offset := 0

	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		tag := data[offset]
		fieldNumber := tag >> 3
		wireType := tag & 0x07
		offset++

		switch fieldNumber {
		case 1: // type
			if wireType == 0 { // Varint
				value, newOffset := readVarint(data, offset)
				if newOffset != -1 {
					hw.Type = RemoteHardwareType(value)
				}
				offset = newOffset
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}

		case 2: // gpio_mask
			if wireType == 0 { // Varint
				value, newOffset := readVarint(data, offset)
				if newOffset != -1 {
					hw.GpioMask = value
				}
				offset = newOffset
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}

		case 3: // gpio_value
			if wireType == 0 { // Varint
				value, newOffset := readVarint(data, offset)
				if newOffset != -1 {
					hw.GpioValue = value
				}
				offset = newOffset
			} else {
				offset = skipPositionField(data, offset, int(wireType))
			}

		default:
			// Skip unknown fields
			offset = skipPositionField(data, offset, int(wireType))
		}

		if offset == -1 {
			break
		}
	}

	return hw
}

// NewWriteGpiosMessage creates a RemoteHardware message for writing GPIO values
func NewWriteGpiosMessage(gpioPins []int, values []bool) *RemoteHardwareMessage {
	if len(gpioPins) != len(values) {
		return nil
	}

	msg := &RemoteHardwareMessage{
		Type: RemoteHardwareWriteGpios,
	}

	for i, pin := range gpioPins {
		if pin < 0 || pin > 63 {
			continue // Skip invalid pins
		}
		msg.GpioMask |= (1 << pin)
		if values[i] {
			msg.GpioValue |= (1 << pin)
		}
	}

	return msg
}

// NewWatchGpiosMessage creates a RemoteHardware message for watching GPIO pins
func NewWatchGpiosMessage(gpioPins []int) *RemoteHardwareMessage {
	msg := &RemoteHardwareMessage{
		Type: RemoteHardwareWatchGpios,
	}

	for _, pin := range gpioPins {
		if pin < 0 || pin > 63 {
			continue // Skip invalid pins
		}
		msg.GpioMask |= (1 << pin)
	}

	return msg
}

// NewReadGpiosMessage creates a RemoteHardware message for reading GPIO pins
func NewReadGpiosMessage(gpioPins []int) *RemoteHardwareMessage {
	msg := &RemoteHardwareMessage{
		Type: RemoteHardwareReadGpios,
	}

	for _, pin := range gpioPins {
		if pin < 0 || pin > 63 {
			continue // Skip invalid pins
		}
		msg.GpioMask |= (1 << pin)
	}

	return msg
}

// containsPositionKeywords checks if payload contains position-related keywords
func containsPositionKeywords(payload []byte) bool {
	text := string(payload)
	keywords := []string{"lat", "lon", "altitude", "position", "gps", "coords", "location"}
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(text), keyword) {
			return true
		}
	}
	return false
}
