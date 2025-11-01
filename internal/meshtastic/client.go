package meshtastic

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Connection interface for abstracted connections
type Connection interface {
	Connect() error
	Close() error
	IsConnected() bool
	GetConnectionInfo() string
	StartPacketListener(handler func([]byte) error) error
	SendCommand(command string) error
}

// Client represents a Meshtastic client that handles protocol communication
type Client struct {
	connection  Connection
	logger      *log.Logger
	packets     chan *Packet
	subscribers []PacketSubscriber
	mu          sync.RWMutex
	stats       *Statistics
	started     bool
	nodeDB      *NodeDB
}

// PacketSubscriber defines the interface for packet subscribers
type PacketSubscriber interface {
	OnPacket(*Packet)
}

// PacketSubscriberFunc is a function adapter for PacketSubscriber
type PacketSubscriberFunc func(*Packet)

func (f PacketSubscriberFunc) OnPacket(p *Packet) {
	f(p)
}

// nodeIDPattern matches Meshtastic node IDs in various formats
var nodeIDPattern = regexp.MustCompile(`!([0-9a-fA-F]{8})|0x([0-9a-fA-F]{8})`)

// Statistics holds packet statistics
type Statistics struct {
	TotalPackets     uint64                `json:"total_packets"`
	PacketsByType    map[PacketType]uint64 `json:"packets_by_type"`
	PacketsByChannel map[uint8]uint64      `json:"packets_by_channel"`
	AverageRSSI      float32               `json:"average_rssi"`
	AverageSNR       float32               `json:"average_snr"`
	StartTime        time.Time             `json:"start_time"`
	LastPacketTime   time.Time             `json:"last_packet_time"`
	mu               sync.RWMutex
}

// NewClient creates a new Meshtastic client
func NewClient(conn Connection, logger *log.Logger) (*Client, error) {
	client := &Client{
		connection: conn,
		logger:     logger,
		packets:    make(chan *Packet, 100), // Buffer for packets
		stats: &Statistics{
			PacketsByType:    make(map[PacketType]uint64),
			PacketsByChannel: make(map[uint8]uint64),
			StartTime:        time.Now(),
		},
		nodeDB: NewNodeDB(),
	}

	return client, nil
}

// Start begins listening for packets from the serial connection
func (c *Client) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return fmt.Errorf("client already started")
	}

	c.logger.Println("Starting Meshtastic client...")

	// Start the packet listener
	c.logger.Printf("Starting packet listener goroutine...")
	go func() {
		c.logger.Printf("Packet listener goroutine started, calling StartPacketListener...")
		if err := c.connection.StartPacketListener(c.handleRawData); err != nil {
			c.logger.Printf("Packet listener error: %v", err)
		} else {
			c.logger.Printf("Packet listener completed successfully")
		}
	}()

	// Start the packet processor
	go c.processPackets()

	c.started = true
	c.logger.Println("Meshtastic client started successfully")

	return nil
}

// Stop stops the client
func (c *Client) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return nil
	}

	c.logger.Println("Stopping Meshtastic client...")
	close(c.packets)
	c.started = false

	return nil
}

// Subscribe adds a packet subscriber
func (c *Client) Subscribe(subscriber PacketSubscriber) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscribers = append(c.subscribers, subscriber)
}

// SubscribeFunc adds a function-based packet subscriber
func (c *Client) SubscribeFunc(fn func(*Packet)) {
	c.Subscribe(PacketSubscriberFunc(fn))
}

// GetStatistics returns current packet statistics
func (c *Client) GetStatistics() *Statistics {
	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := &Statistics{
		TotalPackets:     c.stats.TotalPackets,
		PacketsByType:    make(map[PacketType]uint64),
		PacketsByChannel: make(map[uint8]uint64),
		AverageRSSI:      c.stats.AverageRSSI,
		AverageSNR:       c.stats.AverageSNR,
		StartTime:        c.stats.StartTime,
		LastPacketTime:   c.stats.LastPacketTime,
	}

	for k, v := range c.stats.PacketsByType {
		stats.PacketsByType[k] = v
	}
	for k, v := range c.stats.PacketsByChannel {
		stats.PacketsByChannel[k] = v
	}

	return stats
}

// SendTextMessage sends a text message to a specific node
func (c *Client) SendTextMessage(to uint32, message string) error {
	if !c.connection.IsConnected() {
		return fmt.Errorf("connection not available")
	}

	// In a real implementation, this would format and send a proper Meshtastic packet
	// For now, we'll send a simple command
	cmd := fmt.Sprintf("--sendtext %s", message)
	if to != 0xFFFFFFFF {
		cmd = fmt.Sprintf("--dest !%08x %s", to, cmd)
	}

	return c.connection.SendCommand(cmd)
}

// RequestNodeInfo requests node information from a specific node
func (c *Client) RequestNodeInfo(nodeID uint32) error {
	if !c.connection.IsConnected() {
		return fmt.Errorf("connection not available")
	}

	cmd := fmt.Sprintf("--dest !%08x --request-node-info", nodeID)
	return c.connection.SendCommand(cmd)
}

// SetDebugMode enables or disables debug mode on the device
func (c *Client) SetDebugMode(enabled bool) error {
	if !c.connection.IsConnected() {
		return fmt.Errorf("connection not available")
	}

	var cmd string
	if enabled {
		cmd = "--set debug_log_enabled true"
	} else {
		cmd = "--set debug_log_enabled false"
	}

	return c.connection.SendCommand(cmd)
}

// GetNodeDB returns the node database
func (c *Client) GetNodeDB() *NodeDB {
	return c.nodeDB
}

// GetNodeName returns the friendly name for a node ID
func (c *Client) GetNodeName(nodeID uint32) string {
	return c.nodeDB.GetNodeName(nodeID)
}

// GetNodeShortName returns the short name for a node ID
func (c *Client) GetNodeShortName(nodeID uint32) string {
	return c.nodeDB.GetNodeShortName(nodeID)
}

// handleRawData processes raw data from the connection (binary or JSON)
func (c *Client) handleRawData(data []byte) error {
	c.logger.Printf("Received %d bytes of raw data: %X", len(data), data[:min(len(data), 32)])

	// First, try to parse as JSON (for WiFi connections with synthetic data)
	if packet, err := c.parseJSONPacket(data); err == nil {
		c.logger.Printf("Parsed JSON packet successfully")
		// Send packet to processing channel
		select {
		case c.packets <- packet:
			// Successfully queued
		default:
			c.logger.Println("Packet queue full, dropping packet")
		}
		return nil
	}

	// Try to parse as FromRadio protobuf message (for TCP connections)
	if packet, err := c.parseFromRadioMessage(data); err == nil {
		c.logger.Printf("Parsed FromRadio message successfully: Type=%s, From=%s, To=%s",
			packet.GetTypeName(), packet.GetFromHex(), packet.GetToHex())
		// Send packet to processing channel
		select {
		case c.packets <- packet:
			// Successfully queued
		default:
			c.logger.Println("Packet queue full, dropping packet")
		}
		return nil
	}

	// Try to parse as a binary Meshtastic packet (for serial connections)
	packet, err := ParseRawPacket(data)
	if err != nil {
		c.logger.Printf("Failed to parse as binary packet: %v", err)
		// Try to handle as text data (CLI output, etc.)
		return c.handleTextData(data)
	}

	c.logger.Printf("Parsed packet: Type=%s, From=%s, To=%s, PayloadLen=%d",
		packet.GetTypeName(), packet.GetFromHex(), packet.GetToHex(), len(packet.Payload))

	// Send packet to processing channel
	select {
	case c.packets <- packet:
		// Successfully queued
	default:
		c.logger.Println("Packet queue full, dropping packet")
	}

	return nil
}

// handleTextData processes text-based data from the device
func (c *Client) handleTextData(data []byte) error {
	text := string(data)
	c.logger.Printf("Received text data: %s", strings.TrimSpace(text))

	// Skip empty or whitespace-only messages
	trimmed := strings.TrimSpace(text)
	if len(trimmed) == 0 {
		return nil
	}

	// Try to parse as JSON (for WiFi API responses)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return c.handleJSONData([]byte(trimmed))
	}

	// Create a text packet for CLI output
	packetType := PacketTypeText
	from := uint32(0)
	to := uint32(0xFFFFFFFF)

	// Enhanced packet type detection from text content
	lowerText := strings.ToLower(text)

	// Device startup and configuration patterns
	if strings.Contains(lowerText, "connected to") || strings.Contains(lowerText, "starting up") ||
		strings.Contains(lowerText, "device info") || strings.Contains(lowerText, "firmware") {
		packetType = PacketTypeAdmin
	} else if strings.Contains(lowerText, "channel") && (strings.Contains(lowerText, "settings") || strings.Contains(lowerText, "config")) {
		packetType = PacketTypeAdmin
	} else if strings.Contains(lowerText, "module") && strings.Contains(lowerText, "config") {
		packetType = PacketTypeAdmin
	} else if strings.Contains(lowerText, "preferences") || strings.Contains(lowerText, "setting") {
		packetType = PacketTypeAdmin
	} else if strings.Contains(lowerText, "my info") || strings.Contains(lowerText, "node id") ||
		strings.Contains(lowerText, "owner") || strings.Contains(text, "!!") {
		packetType = PacketTypeNodeInfo
		// Check if this contains node database info
		if nodeID := c.extractNodeInfoFromText(text); nodeID != 0 {
			// Extract and store basic node info from text
			c.extractAndStoreNodeInfoFromText(text, nodeID)
		}
	} else if strings.Contains(lowerText, "rx:") || strings.Contains(lowerText, "tx:") {
		packetType = PacketTypeRouting
	} else if strings.Contains(lowerText, "position") || strings.Contains(lowerText, "gps") ||
		strings.Contains(lowerText, "lat") || strings.Contains(lowerText, "lon") {
		packetType = PacketTypePosition
	} else if strings.Contains(lowerText, "battery") || strings.Contains(lowerText, "voltage") ||
		strings.Contains(lowerText, "telemetry") || strings.Contains(lowerText, "sensor") {
		packetType = PacketTypeTelemetry
	} else if strings.Contains(lowerText, "admin") || strings.Contains(lowerText, "config") {
		packetType = PacketTypeAdmin
	} else if strings.Contains(lowerText, "range") && strings.Contains(lowerText, "test") {
		packetType = PacketTypeRangeTest
	}

	// Try to extract node IDs from text
	if matches := nodeIDPattern.FindAllString(text, -1); len(matches) >= 2 {
		// Found potential from/to addresses
		if id, err := parseNodeID(matches[0]); err == nil {
			from = id
		}
		if id, err := parseNodeID(matches[1]); err == nil {
			to = id
		}
	} else if len(matches) == 1 {
		if id, err := parseNodeID(matches[0]); err == nil {
			from = id
		}
	}

	packet := &Packet{
		ID:     0,
		From:   from,
		To:     to,
		Type:   packetType,
		RxTime: time.Now(),
		DecodedData: NewTextData(trimmed),
		Raw: data,
	}

	select {
	case c.packets <- packet:
		// Successfully queued
	default:
		c.logger.Println("Packet queue full, dropping text packet")
	}

	return nil
}

// handleJSONData processes JSON-formatted data from the device
func (c *Client) handleJSONData(data []byte) error {
	c.logger.Printf("Received JSON data: %s", string(data))

	// For now, treat JSON data as text packets
	// In a full implementation, you'd parse specific JSON packet formats
	packet := &Packet{
		ID:     0,
		From:   0,
		To:     0xFFFFFFFF,
		Type:   PacketTypeText,
		RxTime: time.Now(),
		DecodedData: NewTextData(string(data)),
		Raw: data,
	}

	select {
	case c.packets <- packet:
		// Successfully queued
	default:
		c.logger.Println("Packet queue full, dropping JSON packet")
	}

	return nil
}

// parseJSONPacket attempts to parse JSON data from WiFi synthetic packets
func (c *Client) parseJSONPacket(data []byte) (*Packet, error) {
	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}

	// Check if this looks like our synthetic packet format
	packetType, hasType := jsonData["type"]
	if !hasType {
		return nil, fmt.Errorf("JSON data missing type field")
	}

	typeStr, ok := packetType.(string)
	if !ok {
		return nil, fmt.Errorf("type field is not a string")
	}

	switch typeStr {
	case "device_status":
		return c.parseDeviceStatusPacket(jsonData)
	default:
		return nil, fmt.Errorf("unknown JSON packet type: %s", typeStr)
	}
}

// parseFromRadioMessage parses a FromRadio protobuf message from TCP stream
// This implements the parsing of messages received via Python CLI --listen equivalent
func (c *Client) parseFromRadioMessage(data []byte) (*Packet, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("FromRadio message too short: %d bytes", len(data))
	}

	c.logger.Printf("Parsing FromRadio message: %d bytes, preview: %X", len(data), data[:min(len(data), 8)])

	// Basic protobuf field parsing for FromRadio message
	// This is a simplified parser - in production you'd use proper protobuf libraries
	packet := &Packet{
		RxTime: time.Now(),
		Raw:    data,
	}

	// Parse protobuf fields
	offset := 0
	for offset < len(data) {
		if offset+1 >= len(data) {
			break
		}

		// Read field tag and wire type
		tag := data[offset]
		fieldNumber := tag >> 3
		wireType := tag & 0x07

		offset++

		c.logger.Printf("  Field %d, wire type %d at offset %d", fieldNumber, wireType, offset-1)

		switch fieldNumber {
		case 2: // packet field in FromRadio
			if wireType == 2 { // Length-delimited
				length, newOffset := c.readVarintAt(data, offset)
				if newOffset == -1 || int(newOffset)+int(length) > len(data) {
					return nil, fmt.Errorf("invalid packet field length")
				}
				packetData := data[newOffset : newOffset+int(length)]
				c.logger.Printf("  Found packet data: %d bytes", len(packetData))
				
				// Parse the MeshPacket within the FromRadio
				if err := c.parseMeshPacket(packet, packetData); err != nil {
					c.logger.Printf("  Failed to parse MeshPacket: %v", err)
				} else {
					c.logger.Printf("  Successfully parsed MeshPacket")
				}
				offset = newOffset + int(length)
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 3: // my_info field
			if wireType == 2 { // Length-delimited
				c.logger.Printf("  Found my_info field (device info)")
				length, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 && int(newOffset)+int(length) <= len(data) {
					myInfoData := data[newOffset : newOffset+int(length)]
					c.logger.Printf("  MyInfo data: %d bytes", len(myInfoData))
					// Create a synthetic packet for device info
					packet.Type = PacketTypeNodeInfo
					packet.From = 0 // Local device
					packet.To = 0xFFFFFFFF
					packet.DecodedData = &NodeInfo{
						ID:        "LOCAL_DEVICE",
						LongName:  "My Device Info",
						ShortName: "MINE",
					}
				}
				offset = newOffset + int(length)
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 4: // node_info field  
			if wireType == 2 { // Length-delimited
				c.logger.Printf("  Found node_info field")
				length, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 && int(newOffset)+int(length) <= len(data) {
					nodeInfoData := data[newOffset : newOffset+int(length)]
					c.logger.Printf("  NodeInfo data: %d bytes, hex: %X", len(nodeInfoData), nodeInfoData[:min(len(nodeInfoData), 32)])
					
					// Try to parse the NodeInfo protobuf data
					if nodeInfo := c.parseNodeInfoData(nodeInfoData); nodeInfo != nil {
						c.logger.Printf("  Successfully parsed NodeInfo: %s (%s)", nodeInfo.LongName, nodeInfo.ShortName)
						packet.Type = PacketTypeNodeInfo
						packet.From = 0 // Device sending node DB info
						packet.To = 0xFFFFFFFF
						packet.DecodedData = nodeInfo
					} else {
						c.logger.Printf("  Failed to parse NodeInfo data")
						// Create a text packet with hex data for debugging
						packet.Type = PacketTypeText
						packet.From = 0
						packet.To = 0xFFFFFFFF
						packet.DecodedData = NewTextData(fmt.Sprintf("NodeInfo data: %X", nodeInfoData))
					}
				}
				offset = newOffset + int(length)
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 5: // config field
			if wireType == 2 { // Length-delimited
				c.logger.Printf("  Found config field")
				length, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 && int(newOffset)+int(length) <= len(data) {
					configData := data[newOffset : newOffset+int(length)]
					c.logger.Printf("  Config data: %d bytes", len(configData))
					packet.Type = PacketTypeAdmin
					packet.From = 0
					packet.To = 0xFFFFFFFF
				}
				offset = newOffset + int(length)
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 6: // log_record field
			if wireType == 2 { // Length-delimited
				c.logger.Printf("  Found log_record field")
				length, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 && int(newOffset)+int(length) <= len(data) {
					logData := data[newOffset : newOffset+int(length)]
					c.logger.Printf("  Log record: %d bytes", len(logData))
					packet.Type = PacketTypeText
					packet.From = 0
					packet.To = 0xFFFFFFFF
					packet.DecodedData = &TextData{
						Text: "[LOG] Device log record",
					}
				}
				offset = newOffset + int(length)
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		default:
			// Skip unknown fields
			offset = c.skipField(data, offset, int(wireType))
		}

		if offset == -1 {
			return nil, fmt.Errorf("error parsing FromRadio message")
		}
	}

	// Set defaults if not parsed from packet
	if packet.Type == 0 {
		packet.Type = PacketTypeUnknown
	}
	if packet.To == 0 {
		packet.To = 0xFFFFFFFF // Default to broadcast
	}

	c.logger.Printf("Parsed FromRadio: ID=%d, From=%08x, To=%08x, Type=%s", 
		packet.ID, packet.From, packet.To, packet.GetTypeName())

	return packet, nil
}

// parseMeshPacket parses a MeshPacket from within a FromRadio message
func (c *Client) parseMeshPacket(packet *Packet, data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("MeshPacket too short: %d bytes", len(data))
	}

	c.logger.Printf("    Parsing MeshPacket: %d bytes, hex: %X", len(data), data[:min(len(data), 32)])

	offset := 0
	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		tag := data[offset]
		fieldNumber := tag >> 3
		wireType := tag & 0x07
		c.logger.Printf("      Tag: 0x%02X, Field: %d, WireType: %d at offset %d", tag, fieldNumber, wireType, offset)
		offset++

		switch fieldNumber {
		case 1: // from
			if wireType == 0 { // Varint
				c.logger.Printf("        Parsing From field (varint) at offset %d", offset)
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					packet.From = uint32(value)
					c.logger.Printf("        From: %08x (value: %d, new offset: %d)", packet.From, value, newOffset)
				} else {
					c.logger.Printf("        Failed to read varint for From field")
				}
				offset = newOffset
			} else if wireType == 5 { // Fixed32
				c.logger.Printf("        Parsing From field (fixed32) at offset %d", offset)
				if offset+4 <= len(data) {
					packet.From = binary.LittleEndian.Uint32(data[offset:offset+4])
					c.logger.Printf("        From: %08x (fixed32, new offset: %d)", packet.From, offset+4)
					offset += 4
				} else {
					c.logger.Printf("        Not enough data for fixed32 From field")
					offset = len(data) // Skip to end
				}
			} else {
				c.logger.Printf("        Skipping From field with wire type %d", wireType)
				offset = c.skipField(data, offset, int(wireType))
			}

		case 2: // to  
			if wireType == 0 { // Varint
				c.logger.Printf("        Parsing To field (varint) at offset %d", offset)
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					packet.To = uint32(value)
					c.logger.Printf("        To: %08x (value: %d, new offset: %d)", packet.To, value, newOffset)
				} else {
					c.logger.Printf("        Failed to read varint for To field")
				}
				offset = newOffset
			} else if wireType == 5 { // Fixed32
				c.logger.Printf("        Parsing To field (fixed32) at offset %d", offset)
				if offset+4 <= len(data) {
					packet.To = binary.LittleEndian.Uint32(data[offset:offset+4])
					c.logger.Printf("        To: %08x (fixed32, new offset: %d)", packet.To, offset+4)
					offset += 4
				} else {
					c.logger.Printf("        Not enough data for fixed32 To field")
					offset = len(data) // Skip to end
				}
			} else {
				c.logger.Printf("        Skipping To field with wire type %d", wireType)
				offset = c.skipField(data, offset, int(wireType))
			}

		case 3: // channel
			if wireType == 0 { // Varint
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					packet.Channel = uint8(value)
					c.logger.Printf("      Channel: %d", packet.Channel)
				}
				offset = newOffset
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 6: // id
			if wireType == 0 { // Varint
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					packet.ID = uint32(value)
					c.logger.Printf("      ID: %d", packet.ID)
				}
				offset = newOffset
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 7: // rx_time
			if wireType == 0 { // Varint
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					packet.RxTime = time.Unix(int64(value), 0)
					c.logger.Printf("        RxTime: %v (value: %d)", packet.RxTime, value)
				}
				offset = newOffset
			} else if wireType == 1 { // Fixed64
				if offset+8 <= len(data) {
					timestamp := binary.LittleEndian.Uint64(data[offset:offset+8])
					packet.RxTime = time.Unix(int64(timestamp), 0)
					c.logger.Printf("        RxTime: %v (fixed64)", packet.RxTime)
				}
				offset += 8
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 8: // rx_snr
			if wireType == 5 { // Fixed32 (float)
				if offset+4 <= len(data) {
					bits := binary.LittleEndian.Uint32(data[offset:offset+4])
					packet.RxSNR = math.Float32frombits(bits)
					c.logger.Printf("        RxSNR: %.2f", packet.RxSNR)
					offset += 4
				} else {
					c.logger.Printf("        Not enough data for float RxSNR field")
					offset = len(data)
				}
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 9: // hop_limit
			if wireType == 0 { // Varint
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					packet.HopLimit = uint8(value)
					c.logger.Printf("        HopLimit: %d", packet.HopLimit)
				}
				offset = newOffset
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 10: // want_ack
			if wireType == 0 { // Varint
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					packet.WantAck = value != 0
					c.logger.Printf("        WantAck: %t", packet.WantAck)
				}
				offset = newOffset
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 11: // priority
			if wireType == 0 { // Varint
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					packet.Priority = uint8(value)
					c.logger.Printf("        Priority: %d", packet.Priority)
				}
				offset = newOffset
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 12: // rx_rssi
			if wireType == 0 { // Varint
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					packet.RxRSSI = int32(value)
					c.logger.Printf("        RxRSSI: %d dBm", packet.RxRSSI)
				}
				offset = newOffset
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 14: // via_mqtt
			if wireType == 0 { // Varint
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					viaMqtt := value != 0
					c.logger.Printf("        ViaMqtt: %t", viaMqtt)
				}
				offset = newOffset
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 15: // hop_start
			if wireType == 0 { // Varint
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					hopStart := uint8(value)
					c.logger.Printf("        HopStart: %d", hopStart)
				}
				offset = newOffset
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 16: // public_key
			if wireType == 2 { // Length-delimited
				length, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 && int(newOffset)+int(length) <= len(data) {
					publicKey := data[newOffset : newOffset+int(length)]
					c.logger.Printf("        PublicKey: %d bytes", len(publicKey))
				}
				offset = newOffset + int(length)
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 17: // pki_encrypted
			if wireType == 0 { // Varint
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					pkiEncrypted := value != 0
					c.logger.Printf("        PkiEncrypted: %t", pkiEncrypted)
				}
				offset = newOffset
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 4: // decoded field - contains Data protobuf message
			if wireType == 2 { // Length-delimited
				c.logger.Printf("        Parsing decoded field at offset %d", offset)
				length, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 && int(newOffset)+int(length) <= len(data) {
					dataMsg := data[newOffset : newOffset+int(length)]
					c.logger.Printf("        Data message: %d bytes: %X", len(dataMsg), dataMsg[:min(len(dataMsg), 32)])
					// Parse the Data protobuf message
					if err := c.parseDataMessage(packet, dataMsg); err != nil {
						c.logger.Printf("        Failed to parse Data message: %v", err)
						// Fallback to old method
						packet.Payload = dataMsg
						packet.Type = inferPacketType(dataMsg)
						packet.DecodedData = decodePayload(packet.Type, dataMsg)
					} else {
						c.logger.Printf("        Successfully parsed Data message, type: %s", packet.GetTypeName())
					}
				}
				offset = newOffset + int(length)
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		default:
			// Skip unknown fields
			offset = c.skipField(data, offset, int(wireType))
		}

		if offset == -1 {
			return fmt.Errorf("error parsing MeshPacket")
		}
	}

	return nil
}

// parseDataMessage parses a Data protobuf message and extracts portnum and payload
func (c *Client) parseDataMessage(packet *Packet, data []byte) error {
	if len(data) < 2 {
		return fmt.Errorf("Data message too short: %d bytes", len(data))
	}

	c.logger.Printf("      Parsing Data message: %d bytes", len(data))

	var portnum uint32
	var payload []byte
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
		case 1: // portnum
			if wireType == 0 { // Varint
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					portnum = uint32(value)
					c.logger.Printf("        PortNum: %d", portnum)
				}
				offset = newOffset
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 2: // payload
			if wireType == 2 { // Length-delimited
				length, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 && int(newOffset)+int(length) <= len(data) {
					payload = data[newOffset : newOffset+int(length)]
					c.logger.Printf("        Payload: %d bytes: %X", len(payload), payload[:min(len(payload), 32)])
				}
				offset = newOffset + int(length)
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		default:
			// Skip unknown fields
			offset = c.skipField(data, offset, int(wireType))
		}

		if offset == -1 {
			return fmt.Errorf("error parsing Data message")
		}
	}

	// Map portnum to packet type
	if packetType, exists := PortNumToPacketType[portnum]; exists {
		packet.Type = packetType
		c.logger.Printf("        Mapped portnum %d to type %s", portnum, packet.GetTypeName())
	} else {
		packet.Type = PacketTypeUnknown
		c.logger.Printf("        Unknown portnum %d, using UNKNOWN type", portnum)
	}

	// Set the actual payload from the Data message
	packet.Payload = payload
	packet.DecodedData = decodePayload(packet.Type, payload)

	return nil
}

// Helper methods for protobuf parsing
func (c *Client) readVarintAt(data []byte, offset int) (uint64, int) {
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

func (c *Client) skipField(data []byte, offset int, wireType int) int {
	switch wireType {
	case 0: // Varint
		_, newOffset := c.readVarintAt(data, offset)
		return newOffset
	case 1: // Fixed64
		return offset + 8
	case 2: // Length-delimited
		length, newOffset := c.readVarintAt(data, offset)
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

// parseDeviceStatusPacket creates a packet from device status JSON
func (c *Client) parseDeviceStatusPacket(jsonData map[string]interface{}) (*Packet, error) {
	// Extract timestamp
	timestamp, _ := jsonData["timestamp"].(float64)
	rxTime := time.Unix(int64(timestamp), 0)

	// Extract device info
	deviceInfo, hasDeviceInfo := jsonData["device_info"].(map[string]interface{})
	if !hasDeviceInfo {
		return nil, fmt.Errorf("device_status packet missing device_info")
	}

	// Create a telemetry-like packet from the device status
	packet := &Packet{
		ID:       0,
		From:     0x12345678, // Use a synthetic node ID
		To:       0xFFFFFFFF, // Broadcast
		Type:     PacketTypeTelemetry,
		Channel:  0,
		HopCount: 0,
		HopLimit: 3,
		RxTime:   rxTime,
		Raw:      []byte(fmt.Sprintf("Device Status: %v", deviceInfo)),
	}

	// Extract telemetry data from device info
	telemetry := &TelemetryData{
		Time: uint32(timestamp),
		DeviceMetrics: &DeviceMetrics{},
	}

	// Extract power information if available
	if power, hasPower := deviceInfo["power"].(map[string]interface{}); hasPower {
		if battPct, hasBatt := power["battery_percent"]; hasBatt {
			if battFloat, ok := battPct.(float64); ok {
				telemetry.DeviceMetrics.BatteryLevel = uint32(battFloat)
			}
		}
		if voltage, hasVolt := power["battery_voltage_mv"]; hasVolt {
			if voltFloat, ok := voltage.(float64); ok {
				telemetry.DeviceMetrics.Voltage = float32(voltFloat) / 1000.0 // Convert mV to V
			}
		}
	}

	// Extract airtime information if available
	if airtime, hasAirtime := deviceInfo["airtime"].(map[string]interface{}); hasAirtime {
		if chanUtil, hasChanUtil := airtime["channel_utilization"]; hasChanUtil {
			if utilFloat, ok := chanUtil.(float64); ok {
				telemetry.DeviceMetrics.ChannelUtilization = float32(utilFloat)
			}
		}
		if txUtil, hasTxUtil := airtime["utilization_tx"]; hasTxUtil {
			if txFloat, ok := txUtil.(float64); ok {
				telemetry.DeviceMetrics.AirUtilTx = float32(txFloat)
			}
		}
	}

	// Extract WiFi signal information if available
	if wifi, hasWifi := deviceInfo["wifi"].(map[string]interface{}); hasWifi {
		if rssi, hasRssi := wifi["rssi"]; hasRssi {
			if rssiFloat, ok := rssi.(float64); ok {
				packet.RxRSSI = int32(rssiFloat)
			}
		}
	}

	packet.DecodedData = telemetry
	return packet, nil
}

// processPackets processes packets from the queue
func (c *Client) processPackets() {
	for packet := range c.packets {
		// Update statistics
		c.updateStatistics(packet)

		// Update NodeDB with packet information
		c.updateNodeDB(packet)

		// Notify subscribers
		c.mu.RLock()
		for _, subscriber := range c.subscribers {
			go subscriber.OnPacket(packet) // Process in goroutine to avoid blocking
		}
		c.mu.RUnlock()

		c.logger.Printf("Processed packet: From=%s, To=%s, Type=%s",
			packet.GetFromHex(), packet.GetToHex(), packet.GetTypeName())
	}
}

// updateStatistics updates packet statistics
func (c *Client) updateStatistics(packet *Packet) {
	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()

	c.stats.TotalPackets++
	c.stats.PacketsByType[packet.Type]++
	c.stats.PacketsByChannel[packet.Channel]++
	c.stats.LastPacketTime = packet.RxTime

	// Update average RSSI and SNR
	if packet.RxRSSI != 0 {
		if c.stats.AverageRSSI == 0 {
			c.stats.AverageRSSI = float32(packet.RxRSSI)
		} else {
			c.stats.AverageRSSI = (c.stats.AverageRSSI + float32(packet.RxRSSI)) / 2
		}
	}

	if packet.RxSNR != 0 {
		if c.stats.AverageSNR == 0 {
			c.stats.AverageSNR = packet.RxSNR
		} else {
			c.stats.AverageSNR = (c.stats.AverageSNR + packet.RxSNR) / 2
		}
	}
}

// updateNodeDB updates the node database with information from the packet
func (c *Client) updateNodeDB(packet *Packet) {
	// Always track when we heard from this node (could extend NodeDB later)
	if packet.From != 0 {
		c.logger.Printf("Received packet from node %08x", packet.From)
	}

	// Handle specific packet types that contain node information
	switch packet.Type {
	case PacketTypeNodeInfo:
		if nodeInfo, ok := packet.DecodedData.(*NodeInfo); ok {
			c.logger.Printf("Updating NodeDB with node info from node %08x: %s (%s)", packet.From, nodeInfo.LongName, nodeInfo.ShortName)
			
			// Extract node ID from the packet itself, use From field if nodeInfo.ID is not available 
			nodeID := packet.From
			if nodeInfo.ID != "" {
				if parsed, err := parseNodeID(nodeInfo.ID); err == nil {
					nodeID = parsed
				}
			}
			
			// Store node data in simplified NodeDB
			c.nodeDB.AddOrUpdateUserInfo(nodeID, nodeInfo.ID, nodeInfo.LongName, nodeInfo.ShortName)
		}

	case PacketTypePosition:
		if _, ok := packet.DecodedData.(*PositionData); ok {
			c.logger.Printf("Updating NodeDB with position data from node %08x", packet.From)
		}

	case PacketTypeTelemetry:
		if _, ok := packet.DecodedData.(*TelemetryData); ok {
			c.logger.Printf("Updating NodeDB with telemetry data from node %08x", packet.From)
		}
	}
}

// IsConnected returns true if the client is connected and started
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.started && c.connection.IsConnected()
}

// GetConnectionInfo returns connection information
func (c *Client) GetConnectionInfo() string {
	if !c.IsConnected() {
		return "Disconnected"
	}

	return c.connection.GetConnectionInfo()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// parseNodeID extracts node ID from string containing !12345678 or 0x12345678
func parseNodeID(nodeStr string) (uint32, error) {
	// Remove ! or 0x prefix
	nodeStr = strings.TrimPrefix(nodeStr, "!")
	nodeStr = strings.TrimPrefix(nodeStr, "0x")
	nodeStr = strings.TrimPrefix(nodeStr, "0X")

	// Parse as hex
	id, err := strconv.ParseUint(nodeStr, 16, 32)
	if err != nil {
		return 0, err
	}
	return uint32(id), nil
}

// extractNodeInfoFromText attempts to extract a node ID from text output
func (c *Client) extractNodeInfoFromText(text string) uint32 {
	// Look for node ID patterns like "!12345678" or "Node: 0x12345678"
	patterns := []string{
		`!([0-9a-fA-F]{8})`,
		`0x([0-9a-fA-F]{8})`,
		`node.*?([0-9a-fA-F]{8})`,
		`id.*?([0-9a-fA-F]{8})`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			if id, err := parseNodeID(matches[1]); err == nil {
				return id
			}
		}
	}
	
	return 0
}

// extractAndStoreNodeInfoFromText extracts and stores node information from text output
func (c *Client) extractAndStoreNodeInfoFromText(text string, nodeID uint32) {
	// Try to extract name information from various text formats
	longName := ""
	shortName := ""
	id := fmt.Sprintf("!%08x", nodeID)
	
	// Look for name patterns in text
	// Example: "Owner: John Doe (JD)" or "User: Alice Smith" or "Name: Bob"
	namePatterns := []struct {
		pattern string
		longIdx int
		shortIdx int
	}{
		{`(?i)owner:?\s*([^\(\n]+)\s*\(([^\)]+)\)`, 1, 2}, // "Owner: John Doe (JD)"
		{`(?i)user:?\s*([^\(\n]+)\s*\(([^\)]+)\)`, 1, 2},  // "User: Alice Smith (AS)"
		{`(?i)name:?\s*([^\(\n]+)\s*\(([^\)]+)\)`, 1, 2},  // "Name: Bob Jones (BJ)"
		{`(?i)owner:?\s*([^\n]+)`, 1, 0},                   // "Owner: John Doe"
		{`(?i)user:?\s*([^\n]+)`, 1, 0},                    // "User: Alice Smith"
		{`(?i)name:?\s*([^\n]+)`, 1, 0},                    // "Name: Bob"
	}
	
	for _, np := range namePatterns {
		re := regexp.MustCompile(np.pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) > np.longIdx {
			longName = strings.TrimSpace(matches[np.longIdx])
			if np.shortIdx > 0 && len(matches) > np.shortIdx {
				shortName = strings.TrimSpace(matches[np.shortIdx])
			}
			break
		}
	}
	
	// If we extracted any name info, store it
	if longName != "" || shortName != "" {
		c.logger.Printf("Extracted node info from text: %08x -> '%s' (%s)", nodeID, longName, shortName)
		c.nodeDB.AddOrUpdateUserInfo(nodeID, id, longName, shortName)
	}
}

// parseNodeInfoData parses NodeInfo protobuf data from FromRadio messages
func (c *Client) parseNodeInfoData(data []byte) *NodeInfo {
	if len(data) < 4 {
		c.logger.Printf("NodeInfo data too short: %d bytes", len(data))
		return nil
	}

	c.logger.Printf("Parsing NodeInfo protobuf: %d bytes", len(data))
	nodeInfo := &NodeInfo{}
	offset := 0

	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		tag := data[offset]
		fieldNumber := tag >> 3
		wireType := tag & 0x07
		c.logger.Printf("  NodeInfo field %d, wireType %d at offset %d", fieldNumber, wireType, offset)
		offset++

		switch fieldNumber {
		case 1: // num (node number)
			if wireType == 0 { // Varint
				value, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 {
					nodeID := uint32(value)
					c.logger.Printf("    Node ID: %08x", nodeID)
					nodeInfo.ID = fmt.Sprintf("!%08x", nodeID)
				}
				offset = newOffset
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 2: // user field (User protobuf)
			if wireType == 2 { // Length-delimited
				length, newOffset := c.readVarintAt(data, offset)
				if newOffset != -1 && int(newOffset)+int(length) <= len(data) {
					userdata := data[newOffset : newOffset+int(length)]
					c.logger.Printf("    User data: %d bytes", len(userdata))
					
					// Parse the User protobuf
					if user := parseUserMessage(userdata); user != nil {
						nodeInfo.LongName = user.LongName
						nodeInfo.ShortName = user.ShortName
						if nodeInfo.ID == "" && user.ID != "" {
							nodeInfo.ID = user.ID
						}
						nodeInfo.HwModel = user.HwModel
						nodeInfo.Role = user.Role
						nodeInfo.MacAddr = user.MacAddr
						c.logger.Printf("    Parsed user: %s (%s)", user.LongName, user.ShortName)
					}
				}
				offset = newOffset + int(length)
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 3: // position field
			if wireType == 2 { // Length-delimited
				c.logger.Printf("    Found position data (skipping for now)")
				length, newOffset := c.readVarintAt(data, offset)
				offset = newOffset + int(length)
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 4: // snr field
			if wireType == 5 { // Fixed32 (float)
				c.logger.Printf("    Found SNR data (skipping)")
				offset += 4
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		case 5: // last_heard field
			if wireType == 5 { // Fixed32
				c.logger.Printf("    Found last_heard data (skipping)")
				offset += 4
			} else {
				offset = c.skipField(data, offset, int(wireType))
			}

		default:
			c.logger.Printf("    Skipping unknown NodeInfo field %d", fieldNumber)
			offset = c.skipField(data, offset, int(wireType))
		}

		if offset == -1 {
			c.logger.Printf("  Error parsing NodeInfo at offset")
			break
		}
	}

	// Only return if we got some useful data
	if nodeInfo.ID != "" || nodeInfo.LongName != "" || nodeInfo.ShortName != "" {
		return nodeInfo
	}

	return nil
}
