package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"go-mesh/internal/meshtastic"
	"go-mesh/internal/serial"
	"go-mesh/internal/tcp"
	"go-mesh/internal/ui"
	"go-mesh/internal/wifi"
)

// ConnectionType represents the type of connection to use
type ConnectionType int

const (
	ConnectionSerial ConnectionType = iota
	ConnectionWiFi
	ConnectionTCP
)

// Config holds the application configuration
type Config struct {
	// Serial connection
	Port    string
	Baud    int
	// Network connections
	Host    string
	TCPPort int
	UseTCP  bool  // Use TCP protocol buffer stream instead of HTTP/WebSocket
	// Common
	Verbose bool
	Filter  string
}

// GetConnectionType determines the connection type based on configuration
func (c *Config) GetConnectionType() ConnectionType {
	if c.Host != "" {
		if c.UseTCP {
			return ConnectionTCP
		}
		return ConnectionWiFi
	}
	return ConnectionSerial
}

// Debugger represents the main application
type Debugger struct {
	config     *Config
	connection Connection
	meshtastic *meshtastic.Client
	ui         *tea.Program
	logger     *log.Logger
}

// Connection interface abstracts serial and WiFi connections
type Connection interface {
	Connect() error
	Close() error
	IsConnected() bool
	GetConnectionInfo() string
	StartPacketListener(handler func([]byte) error) error
	SendCommand(command string) error
}

// NewDebugger creates a new debugger instance
func NewDebugger(config *Config) *Debugger {
	// Create file logger for debugging (in addition to stderr)
	logFile, err := os.OpenFile("mesh-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logFile = os.Stderr
	}
	
	logger := log.New(logFile, "[MESH-DEBUG] ", log.LstdFlags)
	
	if !config.Verbose {
		// Still log to file even when not verbose
		logger.SetOutput(logFile)
	}

	return &Debugger{
		config: config,
		logger: logger,
	}
}

// Run starts the debugger application
func (d *Debugger) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		d.logger.Println("Received interrupt signal, shutting down...")
		cancel()
	}()

	// Initialize connection (serial or WiFi)
	if err := d.initConnection(); err != nil {
		return fmt.Errorf("failed to initialize connection: %w", err)
	}
	defer d.connection.Close()

	// Initialize Meshtastic client
	if err := d.initMeshtastic(); err != nil {
		return fmt.Errorf("failed to initialize Meshtastic client: %w", err)
	}

	// Initialize and run UI
	if err := d.initUI(); err != nil {
		return fmt.Errorf("failed to initialize UI: %w", err)
	}

	connInfo := d.connection.GetConnectionInfo()
	d.logger.Printf("Starting Meshtastic debugger: %s", connInfo)
	
	// Start the UI in a goroutine
	uiDone := make(chan error, 1)
	go func() {
		_, err := d.ui.Run()
		uiDone <- err
	}()

	// Wait for context cancellation or UI completion
	select {
	case <-ctx.Done():
		d.ui.Quit()
		return nil
	case err := <-uiDone:
		return err
	}
}

func (d *Debugger) initConnection() error {
	switch d.config.GetConnectionType() {
	case ConnectionSerial:
		conn, err := serial.NewConnection(d.config.Port, d.config.Baud, d.logger)
		if err != nil {
			return err
		}
		d.connection = conn
		return d.connection.Connect()

	case ConnectionWiFi:
		conn, err := wifi.NewConnection(d.config.Host, d.config.TCPPort, d.logger)
		if err != nil {
			return err
		}
		d.connection = conn
		return d.connection.Connect()

	case ConnectionTCP:
		conn, err := tcp.NewConnection(d.config.Host, d.config.TCPPort, d.logger)
		if err != nil {
			return err
		}
		d.connection = conn
		return d.connection.Connect()

	default:
		return fmt.Errorf("unsupported connection type")
	}
}

func (d *Debugger) initMeshtastic() error {
	client, err := meshtastic.NewClient(d.connection, d.logger)
	if err != nil {
		return err
	}
	
	d.meshtastic = client
	return nil
}

func (d *Debugger) initUI() error {
	model := ui.NewModel(d.meshtastic, d.config.Filter, d.logger)
	d.ui = tea.NewProgram(model, tea.WithAltScreen())
	return nil
}
