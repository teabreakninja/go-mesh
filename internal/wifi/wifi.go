package wifi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Connection represents a WiFi connection to a Meshtastic device
type Connection struct {
	host     string
	port     int
	logger   *log.Logger
	client   *http.Client
	wsConn   *websocket.Conn
	mu       sync.RWMutex
	closed   bool
	
	// WebSocket connection for real-time data
	wsURL      string
	wsDialer   *websocket.Dialer
	reconnect  bool
}

// MeshtasticWebSocketMessage represents a message from the WebSocket API
type MeshtasticWebSocketMessage struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64          `json:"timestamp"`
}

// MeshtasticPacket represents a packet from the web API
type MeshtasticPacket struct {
	From      uint32                 `json:"from"`
	To        uint32                 `json:"to"`
	Channel   uint8                  `json:"channel"`
	ID        uint32                 `json:"id"`
	RxTime    int64                  `json:"rxTime"`
	HopLimit  uint8                  `json:"hopLimit"`
	Priority  uint8                  `json:"priority"`
	WantAck   bool                   `json:"wantAck"`
	RxSNR     float32               `json:"rxSNR,omitempty"`
	RxRSSI    int32                 `json:"rxRssi,omitempty"`
	Payload   map[string]interface{} `json:"payload"`
	Decoded   map[string]interface{} `json:"decoded,omitempty"`
}

// NodeInfo represents node information from the web API
type NodeInfo struct {
	NodeID    string `json:"nodeId"`
	LongName  string `json:"longName"`
	ShortName string `json:"shortName"`
	HwModel   string `json:"hwModel"`
	Role      string `json:"role"`
	LastSeen  int64  `json:"lastSeen"`
}

// NewConnection creates a new WiFi connection
func NewConnection(host string, port int, logger *log.Logger) (*Connection, error) {
	if host == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	wsDialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
	}

	baseURL := fmt.Sprintf("http://%s:%d", host, port)
	// Legacy firmware doesn't support WebSocket streaming
	wsURL := ""

	conn := &Connection{
		host:     host,
		port:     port,
		logger:   logger,
		client:   client,
		wsDialer: wsDialer,
		wsURL:    wsURL,
		reconnect: true,
	}

	conn.logger.Printf("Created WiFi connection to %s", baseURL)
	return conn, nil
}

// Connect establishes the WiFi connection to the device
func (c *Connection) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("connection is closed")
	}

	// Test HTTP connection first
	if err := c.testHTTPConnection(); err != nil {
		return fmt.Errorf("failed to connect via HTTP: %w", err)
	}

	// Try to establish WebSocket connection (may not be available in legacy firmware)
	if c.wsURL != "" {
		if err := c.connectWebSocket(); err != nil {
			c.logger.Printf("WebSocket not available (legacy firmware): %v", err)
			c.logger.Printf("Will use HTTP polling for packet data")
		}
	} else {
		c.logger.Printf("Legacy firmware detected - WebSocket not supported")
	}

	c.logger.Printf("Successfully connected to Meshtastic device at %s:%d", c.host, c.port)
	return nil
}

// testHTTPConnection tests if the device is reachable via HTTP
func (c *Connection) testHTTPConnection() error {
	// Use legacy JSON API endpoint that works with firmware 2.6.11
	url := fmt.Sprintf("http://%s:%d/json/report", c.host, c.port)
	
	resp, err := c.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to reach device: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("device returned status %d", resp.StatusCode)
	}

	// Verify this is actually a Meshtastic device by checking response content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	
	// Basic validation that this looks like a Meshtastic JSON report
	var report map[string]interface{}
	if err := json.Unmarshal(body, &report); err != nil {
		return fmt.Errorf("device response is not valid JSON: %w", err)
	}
	
	if _, hasData := report["data"]; !hasData {
		return fmt.Errorf("device response doesn't contain expected Meshtastic data")
	}

	c.logger.Printf("HTTP connection test successful - Meshtastic device detected")
	return nil
}

// connectWebSocket establishes WebSocket connection for real-time data
func (c *Connection) connectWebSocket() error {
	header := http.Header{}
	
	conn, _, err := c.wsDialer.Dial(c.wsURL, header)
	if err != nil {
		return fmt.Errorf("failed to dial WebSocket: %w", err)
	}

	c.wsConn = conn
	c.logger.Printf("WebSocket connection established")
	return nil
}

// StartPacketListener starts listening for packets via WebSocket or HTTP polling
func (c *Connection) StartPacketListener(handler func([]byte) error) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("connection not established")
	}
	wsConn := c.wsConn
	c.mu.RUnlock()

	// If WebSocket is available, use it
	if wsConn != nil {
		return c.startWebSocketListener(wsConn, handler)
	}

	// Fallback to HTTP polling for legacy firmware
	return c.startHTTPPollingListener(handler)
}

// startWebSocketListener handles WebSocket-based packet listening
func (c *Connection) startWebSocketListener(wsConn *websocket.Conn, handler func([]byte) error) error {
	c.logger.Printf("Starting WebSocket packet listener")

	for {
		c.mu.RLock()
		if c.closed {
			c.mu.RUnlock()
			break
		}
		c.mu.RUnlock()

		// Set read deadline
		wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))

		messageType, data, err := wsConn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				c.logger.Println("WebSocket connection closed by remote")
				break
			}
			
			// Handle timeout and other errors
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Printf("WebSocket error: %v", err)
			}
			
			// Attempt to reconnect if enabled
			if c.reconnect && !c.closed {
				c.logger.Println("Attempting to reconnect WebSocket...")
				if err := c.reconnectWebSocket(); err != nil {
					c.logger.Printf("Failed to reconnect: %v", err)
					time.Sleep(5 * time.Second) // Wait before next attempt
					continue
				}
				wsConn = c.wsConn
				continue
			}
			break
		}

		if messageType != websocket.TextMessage {
			continue // Skip binary messages for now
		}

		c.logger.Printf("Received WebSocket message: %d bytes", len(data))

		// Parse WebSocket message
		var wsMsg MeshtasticWebSocketMessage
		if err := json.Unmarshal(data, &wsMsg); err != nil {
			c.logger.Printf("Failed to parse WebSocket message: %v", err)
			continue
		}

		// Convert to binary format for consistent handling
		binaryData, err := c.convertToBinary(wsMsg)
		if err != nil {
			c.logger.Printf("Failed to convert message to binary: %v", err)
			continue
		}

		// Process the packet
		if err := handler(binaryData); err != nil {
			c.logger.Printf("Error processing packet: %v", err)
		}
	}

	return nil
}

// reconnectWebSocket attempts to reconnect the WebSocket
func (c *Connection) reconnectWebSocket() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.wsConn != nil {
		c.wsConn.Close()
	}

	return c.connectWebSocket()
}

// convertToBinary converts WebSocket JSON message to binary format
// This allows the existing packet parsing logic to work with WiFi data
func (c *Connection) convertToBinary(wsMsg MeshtasticWebSocketMessage) ([]byte, error) {
	// Parse the data as a Meshtastic packet
	var packet MeshtasticPacket
	if err := json.Unmarshal(wsMsg.Data, &packet); err != nil {
		return nil, fmt.Errorf("failed to parse packet data: %w", err)
	}

	// Create a simple binary representation for compatibility
	// In a real implementation, you might want to use protobuf encoding
	data := make([]byte, 0, 1024)
	
	// Add basic header information (simplified)
	header := struct {
		ID       uint32  `json:"id"`
		From     uint32  `json:"from"`
		To       uint32  `json:"to"`
		Channel  uint8   `json:"channel"`
		HopLimit uint8   `json:"hopLimit"`
		Priority uint8   `json:"priority"`
		RxSNR    float32 `json:"rxSNR"`
		RxRSSI   int32   `json:"rxRSSI"`
	}{
		ID:       packet.ID,
		From:     packet.From,
		To:       packet.To,
		Channel:  packet.Channel,
		HopLimit: packet.HopLimit,
		Priority: packet.Priority,
		RxSNR:    packet.RxSNR,
		RxRSSI:   packet.RxRSSI,
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal header: %w", err)
	}

	data = append(data, headerBytes...)

	// Add payload if present
	if packet.Decoded != nil {
		payloadBytes, err := json.Marshal(packet.Decoded)
		if err == nil {
			data = append(data, payloadBytes...)
		}
	}

	return data, nil
}

// startHTTPPollingListener polls the device for updates (fallback for legacy firmware)
func (c *Connection) startHTTPPollingListener(handler func([]byte) error) error {
	c.logger.Printf("Starting HTTP polling listener (legacy firmware mode)")
	
	// For legacy firmware, we'll periodically poll /json/report
	// This isn't ideal for real-time packet capture, but it's the best we can do
	ticker := time.NewTicker(2 * time.Second) // Poll every 2 seconds
	defer ticker.Stop()
	
	lastReportTime := time.Time{}
	
	for {
		c.mu.RLock()
		if c.closed {
			c.mu.RUnlock()
			break
		}
		c.mu.RUnlock()
		
		select {
		case <-ticker.C:
			// Poll the device for status updates
			if err := c.pollDeviceStatus(&lastReportTime, handler); err != nil {
				c.logger.Printf("Error polling device: %v", err)
			}
		}
	}
	
	return nil
}

// pollDeviceStatus polls /json/report and extracts any new information
func (c *Connection) pollDeviceStatus(lastReportTime *time.Time, handler func([]byte) error) error {
	url := fmt.Sprintf("http://%s:%d/json/report", c.host, c.port)
	
	resp, err := c.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get device status: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	
	var report map[string]interface{}
	if err := json.Unmarshal(body, &report); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}
	
	// Create a synthetic packet from the device report
	// This allows the UI to display device information even without real packets
	currentTime := time.Now()
	if currentTime.Sub(*lastReportTime) > 5*time.Second {
		*lastReportTime = currentTime
		
		// Convert device report to a format the handler can process
		syntheticPacket := c.createSyntheticPacket(report)
		if syntheticPacket != nil {
			if err := handler(syntheticPacket); err != nil {
				c.logger.Printf("Error processing synthetic packet: %v", err)
			}
		}
	}
	
	return nil
}

// createSyntheticPacket creates a packet-like structure from device status
func (c *Connection) createSyntheticPacket(report map[string]interface{}) []byte {
	// Extract useful information from the report
	data, ok := report["data"].(map[string]interface{})
	if !ok {
		return nil
	}
	
	// Create a simplified packet structure
	syntheticData := map[string]interface{}{
		"type": "device_status",
		"timestamp": time.Now().Unix(),
		"device_info": data,
	}
	
	packetBytes, err := json.Marshal(syntheticData)
	if err != nil {
		c.logger.Printf("Failed to marshal synthetic packet: %v", err)
		return nil
	}
	
	return packetBytes
}

// SendCommand sends a command to the device via HTTP API
func (c *Connection) SendCommand(command string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("connection is closed")
	}

	// Parse command and determine appropriate API endpoint
	endpoint, payload, err := c.parseCommand(command)
	if err != nil {
		return fmt.Errorf("failed to parse command: %w", err)
	}

	url := fmt.Sprintf("http://%s:%d%s", c.host, c.port, endpoint)
	
	var resp *http.Response
	if payload != nil {
		// POST request with JSON payload
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}

		resp, err = c.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	} else {
		// GET request
		resp, err = c.client.Get(url)
	}

	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	c.logger.Printf("Sent command successfully: %s", command)
	return nil
}

// parseCommand parses a command string into HTTP API endpoint and payload
func (c *Connection) parseCommand(command string) (string, interface{}, error) {
	// Simple command parsing adapted for legacy firmware (2.6.11)
	
	if command == "--get-status" {
		// Use legacy JSON report endpoint
		return "/json/report", nil, nil
	}
	
	if command == "--get-nodes" {
		// Legacy firmware doesn't have a nodes endpoint
		// Return the general report which contains device info
		return "/json/report", nil, nil
	}

	// Text message command: --sendtext "message"
	// Note: Legacy firmware may not support HTTP message sending
	if len(command) > 11 && command[:10] == "--sendtext" {
		message := command[11:] // Remove "--sendtext "
		message = trimQuotes(message)
		
		// Legacy firmware doesn't typically support HTTP message sending
		// This will likely fail, but we'll try anyway
		payload := map[string]interface{}{
			"text": message,
			"to":   "broadcast",
		}
		return "/json/send", payload, nil
	}

	// Configuration commands
	// Legacy firmware may not support HTTP configuration
	if len(command) > 6 && command[:5] == "--set" {
		// Parse --set key=value
		parts := parseKeyValue(command[6:])
		if len(parts) == 2 {
			// Legacy firmware doesn't typically support HTTP config changes
			return "", nil, fmt.Errorf("configuration changes not supported via HTTP in firmware 2.6.11 - use serial connection")
		}
	}

	return "", nil, fmt.Errorf("unsupported command: %s (note: legacy firmware 2.6.11 has limited HTTP API support)", command)
}

// Close closes the WiFi connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	c.reconnect = false

	if c.wsConn != nil {
		c.wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.wsConn.Close()
	}

	c.logger.Printf("Closed WiFi connection to %s:%d", c.host, c.port)
	return nil
}

// IsConnected returns true if the connection is established
func (c *Connection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// For legacy firmware, we consider connected if HTTP connection works (WebSocket not required)
	return !c.closed
}

// GetConnectionInfo returns connection information string
func (c *Connection) GetConnectionInfo() string {
	if !c.IsConnected() {
		return "Disconnected"
	}
	return fmt.Sprintf("Connected to %s:%d via WiFi", c.host, c.port)
}

// GetNodeInfo retrieves node information from the device (adapted for legacy firmware)
func (c *Connection) GetNodeInfo() ([]NodeInfo, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	// Legacy firmware doesn't have /api/v1/nodes, use /json/report instead
	url := fmt.Sprintf("http://%s:%d/json/report", c.host, c.port)
	
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	// Parse the device report and extract what node info we can
	var report map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return nil, fmt.Errorf("failed to decode device report: %w", err)
	}

	// Create a single NodeInfo entry from the device report
	// Legacy firmware doesn't provide mesh node information via HTTP
	nodes := []NodeInfo{
		{
			NodeID:    "local",
			LongName:  "Local Device",
			ShortName: "LOC",
			HwModel:   "Unknown",
			Role:      "device",
			LastSeen:  time.Now().Unix(),
		},
	}

	return nodes, nil
}

// Helper functions

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseKeyValue(s string) []string {
	if idx := findChar(s, '='); idx != -1 {
		key := trimSpace(s[:idx])
		value := trimSpace(s[idx+1:])
		return []string{key, value}
	}
	return nil
}

func findChar(s string, c rune) int {
	for i, r := range s {
		if r == c {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	
	return s[start:end]
}
