package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go-mesh/internal/app"
)

var (
	// Serial connection options
	port    string
	baud    int
	
	// Network connection options
	host    string
	tcpPort int
	useTCP  bool
	
	// Common options
	verbose bool
	filter  string
)

var rootCmd = &cobra.Command{
	Use:   "mesh-debug",
	Short: "Meshtastic Terminal Debugger",
	Long: `A terminal application for connecting to Meshtastic nodes and debugging radio packets.

This tool provides real-time packet capture, analysis, and debugging capabilities
for Meshtastic mesh networks. Supports multiple connection types:

  Serial: Direct USB/serial connection to device
  WiFi:   HTTP/WebSocket API connection (limited packet visibility)
  TCP:    Protocol buffer stream connection (full RF traffic capture)

For maximum RF traffic visibility, use --tcp flag with --host.`,
	RunE: runDebugger,
}

func init() {
	// Serial connection flags
	rootCmd.Flags().StringVarP(&port, "port", "p", "", "Serial port of Meshtastic device (e.g., COM3)")
	rootCmd.Flags().IntVarP(&baud, "baud", "b", 115200, "Baud rate for serial connection")
	
	// Network connection flags
	rootCmd.Flags().StringVar(&host, "host", "", "IP address or hostname of Meshtastic device (e.g., 192.168.1.100)")
	rootCmd.Flags().IntVar(&tcpPort, "tcp-port", 4403, "Port for network connection (80 for HTTP/WiFi, 4403 for TCP protocol buffer stream)")
	rootCmd.Flags().BoolVar(&useTCP, "tcp", false, "Use TCP protocol buffer stream for full RF traffic (like Python CLI --listen). Requires --host.")
	
	// Common flags
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.Flags().StringVarP(&filter, "filter", "f", "", "Filter packets (node ID, message type, etc.)")
	
	// Make port and host mutually exclusive but one is required
	rootCmd.MarkFlagsRequiredTogether()
}

func runDebugger(cmd *cobra.Command, args []string) error {
	// Validate that either port or host is specified (but not both)
	if port == "" && host == "" {
		return fmt.Errorf("either --port (for serial) or --host (for network) must be specified")
	}
	if port != "" && host != "" {
		return fmt.Errorf("cannot specify both --port and --host, choose either serial or network connection")
	}
	
	// Validate TCP flag usage
	if useTCP && host == "" {
		return fmt.Errorf("--tcp flag requires --host to be specified")
	}
	
	// Set default port based on connection type
	if host != "" && tcpPort == 4403 && !useTCP {
		// If host is specified but not using TCP, default to HTTP port
		tcpPort = 80
	}
	
	config := &app.Config{
		// Serial connection
		Port:    port,
		Baud:    baud,
		// Network connection
		Host:    host,
		TCPPort: tcpPort,
		UseTCP:  useTCP,
		// Common
		Verbose: verbose,
		Filter:  filter,
	}
	
	// Connection info is logged to mesh-debug.log instead of stdout to avoid TUI corruption
	
	debugger := app.NewDebugger(config)
	return debugger.Run()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
