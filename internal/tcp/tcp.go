package tcp

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// Meshtastic stream protocol constants (from Python implementation)
const (
	START1              = 0x94
	START2              = 0xC3
	HEADER_LEN          = 4
	MAX_TO_FROM_RADIO_SIZE = 512
)

// Connection represents a TCP connection to a Meshtastic device
// This implements the stream protocol from Python CLI --listen
type Connection struct {
	host      string
	port      int
	conn      net.Conn
	logger    *log.Logger
	mu        sync.RWMutex
	closed    bool
	connected bool
	
	// Stream protocol state
	rxBuf     []byte
	wantExit  bool
}

// NewConnection creates a new TCP connection for protocol buffer streaming
func NewConnection(host string, port int, logger *log.Logger) (*Connection, error) {
	if host == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}

	conn := &Connection{
		host:     host,
		port:     port,
		logger:   logger,
		rxBuf:    make([]byte, 0),
		wantExit: false,
	}

	conn.logger.Printf("Created TCP connection for %s:%d (Meshtastic stream protocol)", host, port)
	return conn, nil
}

// Connect establishes the TCP connection and sends wake-up sequence
func (c *Connection) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("connection is closed")
	}

	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	c.logger.Printf("Connecting to Meshtastic device at %s for stream protocol", addr)

	// Connect to the TCP port
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.conn = conn
	c.connected = true

	// Send wake-up sequence like Python CLI does
	c.logger.Printf("Sending wake-up sequence (32 x START2 bytes)...")
	wakeUpBytes := make([]byte, 32)
	for i := range wakeUpBytes {
		wakeUpBytes[i] = START2
	}
	
	if err := c.writeBytes(wakeUpBytes); err != nil {
		return fmt.Errorf("failed to send wake-up sequence: %w", err)
	}

	// Wait 100ms like Python CLI
	time.Sleep(100 * time.Millisecond)

	// Send configuration request like Python CLI _startConfig()
	if err := c.startConfig(); err != nil {
		c.logger.Printf("Warning: failed to send config request: %v", err)
	} else {
		c.logger.Printf("Configuration request sent successfully")
	}

	c.logger.Printf("Successfully connected to Meshtastic stream at %s", addr)
	return nil
}

// StartPacketListener starts the stream reader (matches Python CLI --listen)
func (c *Connection) StartPacketListener(handler func([]byte) error) error {
	c.mu.RLock()
	if c.closed || !c.connected {
		c.mu.RUnlock()
		return fmt.Errorf("connection not established")
	}
	c.mu.RUnlock()

	c.logger.Printf("Starting Meshtastic stream reader (Python CLI --listen equivalent)")

	// Start the reader loop - this matches the Python __reader method
	return c.streamReader(handler)
}

// streamReader implements the Python __reader method
func (c *Connection) streamReader(handler func([]byte) error) error {
	c.logger.Printf("Stream reader started")
	
	for !c.wantExit {
		// Read one byte at a time like Python CLI
		buf := make([]byte, 1)
		c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		
		n, err := c.conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout is normal when no data
				continue
			}
			if err == io.EOF {
				c.logger.Println("Connection closed by remote")
				break
			}
			c.logger.Printf("Error reading byte: %v", err)
			break
		}
		
		if n > 0 {
			c.logger.Printf("Read byte: 0x%02X", buf[0])
			if err := c.processByte(buf[0], handler); err != nil {
				c.logger.Printf("Error processing byte: %v", err)
			}
		}
	}
	
	c.logger.Printf("Stream reader exiting")
	return nil
}

// processByte processes a single byte according to Meshtastic stream protocol
func (c *Connection) processByte(b byte, handler func([]byte) error) error {
	ptr := len(c.rxBuf)
	
	// Append byte to buffer (assume we want to append)
	c.rxBuf = append(c.rxBuf, b)
	
	if ptr == 0 {
		// Looking for START1
		if b != START1 {
			c.rxBuf = c.rxBuf[:0] // Reset buffer
			// This might be a log message - ignore for now
			c.logger.Printf("Expected START1 (0x%02X), got 0x%02X - discarding", START1, b)
		}
	} else if ptr == 1 {
		// Looking for START2
		if b != START2 {
			c.rxBuf = c.rxBuf[:0] // Reset buffer
			c.logger.Printf("Expected START2 (0x%02X), got 0x%02X - discarding", START2, b)
		}
	} else if ptr >= HEADER_LEN-1 {
		// We have at least a header
		if ptr == HEADER_LEN-1 {
			// Just finished reading header, validate length
			packetLen := (int(c.rxBuf[2]) << 8) + int(c.rxBuf[3])
			c.logger.Printf("Packet length: %d bytes", packetLen)
			
			if packetLen > MAX_TO_FROM_RADIO_SIZE {
				c.logger.Printf("Packet length %d exceeds maximum %d - discarding", packetLen, MAX_TO_FROM_RADIO_SIZE)
				c.rxBuf = c.rxBuf[:0] // Reset buffer
				return nil
			}
		}
		
		// Check if we have complete packet
		packetLen := (int(c.rxBuf[2]) << 8) + int(c.rxBuf[3])
		if len(c.rxBuf) >= packetLen+HEADER_LEN {
			// Complete packet received
			c.logger.Printf("Complete packet received: %d bytes total, %d bytes payload", len(c.rxBuf), packetLen)
			
			// Extract payload (skip header)
			payload := c.rxBuf[HEADER_LEN : HEADER_LEN+packetLen]
			c.logger.Printf("Payload: %X", payload)
			
			// Process the payload
			if err := handler(payload); err != nil {
				c.logger.Printf("Error handling payload: %v", err)
			}
			
			// Reset buffer for next packet
			c.rxBuf = c.rxBuf[:0]
		}
	}
	
	return nil
}

// startConfig sends a ToRadio configuration request like Python CLI _startConfig()
func (c *Connection) startConfig() error {
	c.logger.Printf("Sending configuration request (ToRadio with want_config_id)")

	// Create ToRadio protobuf message with want_config_id
	// This is what the Python CLI sends to start receiving packets
	toRadioBytes := c.createToRadioWantConfig()

	// Send using Meshtastic stream protocol (START1, START2, length, payload)
	return c.sendToRadio(toRadioBytes)
}

// createToRadioWantConfig creates a ToRadio message with want_config_id
func (c *Connection) createToRadioWantConfig() []byte {
	// ToRadio protobuf message:
	// message ToRadio {
	//   uint32 want_config_id = 3;
	// }
	// Field 3, wire type 0 (varint)
	// Tag = (3 << 3) | 0 = 24 = 0x18

	configID := uint32(0) // Request config with ID 0
	c.logger.Printf("Creating ToRadio message: want_config_id=%d (field 3)", configID)

	// Simple protobuf encoding
	message := make([]byte, 0, 8)

	// Field 3: want_config_id
	// Encode field tag as varint: 24 = 0x18
	message = append(message, 0x18) // Field tag (24)
	message = c.appendVarint(message, uint64(configID)) // Value (0)

	c.logger.Printf("Created ToRadio message: %d bytes: %X", len(message), message)
	return message
}

// sendToRadio sends a ToRadio message using Meshtastic stream protocol
func (c *Connection) sendToRadio(toRadioBytes []byte) error {
	bufLen := len(toRadioBytes)
	c.logger.Printf("Sending ToRadio message: %d bytes", bufLen)

	// Create header: START1, START2, length_high, length_low (big-endian)
	header := []byte{
		START1,
		START2,
		byte((bufLen >> 8) & 0xFF), // High byte
		byte(bufLen & 0xFF),        // Low byte
	}

	c.logger.Printf("Sending header: %X", header)
	c.logger.Printf("Sending payload: %X", toRadioBytes)

	// Send header + payload
	fullMessage := append(header, toRadioBytes...)
	return c.writeBytes(fullMessage)
}

// appendVarint appends a varint-encoded value to a byte slice
func (c *Connection) appendVarint(data []byte, value uint64) []byte {
	for value >= 0x80 {
		data = append(data, byte(value&0x7F|0x80))
		value >>= 7
	}
	data = append(data, byte(value&0x7F))
	return data
}

// writeBytes writes bytes to the connection and flushes
func (c *Connection) writeBytes(data []byte) error {
	if c.conn == nil {
		return fmt.Errorf("connection not established")
	}
	
	_, err := c.conn.Write(data)
	if err != nil {
		return err
	}
	
	// Flush by setting TCP_NODELAY-like behavior
	if tcpConn, ok := c.conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
	}
	
	return nil
}


// SendCommand sends a command to the device
// For TCP connections, this would send a protocol buffer command
func (c *Connection) SendCommand(command string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || !c.connected {
		return fmt.Errorf("connection not available")
	}

	// For now, we'll log that command sending isn't implemented for TCP
	// In a full implementation, you'd encode the command as a protobuf message
	c.logger.Printf("Command sending not yet implemented for TCP connection: %s", command)
	return fmt.Errorf("command sending not implemented for TCP protocol buffer stream")
}

// Close closes the TCP connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.wantExit = true
	c.closed = true
	c.connected = false

	if c.conn != nil {
		c.logger.Printf("Closing TCP connection to %s:%d", c.host, c.port)
		return c.conn.Close()
	}

	return nil
}

// IsConnected returns true if the connection is established
func (c *Connection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && !c.closed
}

// GetConnectionInfo returns connection information string
func (c *Connection) GetConnectionInfo() string {
	if !c.IsConnected() {
		return "Disconnected"
	}
	return fmt.Sprintf("Connected to %s:%d via TCP (Protocol Buffer Stream)", c.host, c.port)
}

