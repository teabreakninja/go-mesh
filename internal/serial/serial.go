package serial

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"go.bug.st/serial"
)

// Connection represents a serial connection to a Meshtastic device
type Connection struct {
	port     serial.Port
	portName string
	baud     int
	reader   *bufio.Reader
	writer   io.Writer
	logger   *log.Logger
	mu       sync.RWMutex
	closed   bool
}


// NewConnection creates a new serial connection
func NewConnection(portName string, baud int, logger *log.Logger) (*Connection, error) {
	conn := &Connection{
		portName: portName,
		baud:     baud,
		logger:   logger,
	}

	conn.logger.Printf("Created serial connection for %s at %d baud", portName, baud)
	return conn, nil
}

// Connect establishes the serial connection
func (c *Connection) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("connection is closed")
	}

	mode := &serial.Mode{
		BaudRate: c.baud,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(c.portName, mode)
	if err != nil {
		return fmt.Errorf("failed to open serial port %s: %w", c.portName, err)
	}

	c.port = port
	c.reader = bufio.NewReader(port)
	c.writer = port

	// Set read timeout
	if err := port.SetReadTimeout(1 * time.Second); err != nil {
		port.Close()
		return fmt.Errorf("failed to set read timeout: %w", err)
	}

	c.logger.Printf("Successfully opened serial port %s at %d baud", c.portName, c.baud)
	return nil
}

// Read reads data from the serial connection
func (c *Connection) Read(buffer []byte) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.closed {
		return 0, fmt.Errorf("connection is closed")
	}
	
	return c.port.Read(buffer)
}

// Write writes data to the serial connection
func (c *Connection) Write(data []byte) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.closed {
		return 0, fmt.Errorf("connection is closed")
	}
	
	n, err := c.writer.Write(data)
	if err != nil {
		c.logger.Printf("Error writing to serial port: %v", err)
		return n, err
	}
	
	c.logger.Printf("Wrote %d bytes to serial port", n)
	return n, nil
}

// ReadLine reads a complete line from the serial connection
func (c *Connection) ReadLine() (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.closed {
		return "", fmt.Errorf("connection is closed")
	}
	
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	
	return line, nil
}

// StartPacketListener starts listening for incoming packets
func (c *Connection) StartPacketListener(handler func([]byte) error) error {
	buffer := make([]byte, 4096)
	
	for {
		c.mu.RLock()
		if c.closed {
			c.mu.RUnlock()
			break
		}
		c.mu.RUnlock()
		
		n, err := c.Read(buffer)
		if err != nil {
			if err == io.EOF {
				c.logger.Println("Serial connection closed by remote")
				break
			}
			// Handle timeout errors gracefully
			if isTimeout(err) {
				continue
			}
			c.logger.Printf("Error reading from serial port: %v", err)
			continue
		}
		
		if n > 0 {
			c.logger.Printf("Received %d bytes from serial port", n)
			
			// Process the packet
			if err := handler(buffer[:n]); err != nil {
				c.logger.Printf("Error processing packet: %v", err)
			}
		}
	}
	
	return nil
}

// SendCommand sends a command to the Meshtastic device
func (c *Connection) SendCommand(command string) error {
	cmd := command + "\n"
	_, err := c.Write([]byte(cmd))
	if err != nil {
		return fmt.Errorf("failed to send command %s: %w", command, err)
	}
	
	c.logger.Printf("Sent command: %s", command)
	return nil
}

// Close closes the serial connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.closed {
		return nil
	}
	
	c.closed = true
	c.logger.Printf("Closing serial connection to %s", c.portName)
	
	return c.port.Close()
}

// IsConnected returns true if the connection is open
func (c *Connection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.closed
}

// GetPortName returns the port name
func (c *Connection) GetPortName() string {
	return c.portName
}

// GetBaudRate returns the baud rate
func (c *Connection) GetBaudRate() int {
	return c.baud
}

// GetConnectionInfo returns information about the connection
func (c *Connection) GetConnectionInfo() string {
	return fmt.Sprintf("Serial %s at %d baud", c.portName, c.baud)
}

// isTimeout checks if the error is a timeout error
func isTimeout(err error) bool {
	// This is a simple check - in practice, you might want more sophisticated timeout detection
	return err != nil && (err.Error() == "timeout" || err.Error() == "read timeout")
}
