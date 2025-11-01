# Meshtastic Terminal Debugger

A Go-based terminal application for connecting to Meshtastic nodes and debugging radio packets.

## Features

- Connect to Meshtastic devices via serial port
- Real-time packet capture and analysis
- Interactive terminal UI with packet filtering
- Debug information including signal strength, hop counts, and timing
- Support for multiple packet types and routing analysis

## Prerequisites

1. **Go Installation** (required)
   - Download and install Go from: https://golang.org/dl/
   - Make sure Go is added to your system PATH
   - Verify installation with: `go version`

2. **Meshtastic Device** (choose one connection method)
   - **Serial/USB Connection**: Meshtastic device connected via USB
   - **WiFi Connection**: Meshtastic device connected to your WiFi network
   - Device should be in developer/debug mode for full packet visibility

## Setup

1. Initialize the Go module:
```bash
go mod init go-mesh
```

2. Install dependencies:
```bash
go mod tidy
```

3. Build the application:
```bash
go build -o mesh-debug.exe ./cmd/mesh-debug
```

4. Run the debugger:
```bash
# Serial connection
./mesh-debug.exe --port COM3  # Replace COM3 with your device's port

# WiFi connection  
./mesh-debug.exe --host 192.168.1.100  # Replace with your device's IP
```

## Usage

**Connection Options (choose one):**
- `--port`: Serial port of your Meshtastic device (e.g., COM3, COM4)
- `--host`: IP address or hostname of your Meshtastic device (e.g., 192.168.1.100)

**Additional Options:**
- `--baud`: Baud rate for serial connection (default: 115200)
- `--tcp-port`: HTTP port for WiFi connection (default: 80)
- `--filter`: Filter packets by node ID or message type
- `--verbose`: Enable verbose logging

## Architecture

- `cmd/mesh-debug/`: Main application entry point
- `internal/meshtastic/`: Meshtastic protocol implementation
- `internal/ui/`: Terminal user interface components
- `internal/serial/`: Serial communication handling
- `internal/packets/`: Packet parsing and analysis

## Development

This project uses:
- Go 1.21+
- Protocol Buffers for Meshtastic message parsing
- Serial communication libraries
- Terminal UI libraries for interactive debugging
