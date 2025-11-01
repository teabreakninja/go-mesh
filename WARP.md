# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Project Overview

This is a Go-based terminal application that connects to Meshtastic nodes for debugging radio packets in real-time. It provides an interactive TUI (Terminal User Interface) built with Bubble Tea for packet capture, analysis, and network debugging of Meshtastic mesh networks.

## Development Commands

### Building and Testing
```bash
# Build the application
go build -o mesh-debug.exe ./cmd/mesh-debug

# Test build with automated validation
./test-build.ps1                # Windows PowerShell script

# Download and manage dependencies
go mod tidy

# Run static analysis
go vet ./...

# Check for Go installation and setup environment
./install-go.ps1              # Windows PowerShell script
```

### Running the Application
```bash
# Serial connection (most common during development)
./mesh-debug --port COM3 --verbose

# WiFi connection (HTTP/WebSocket - limited packet visibility)
./mesh-debug --host 192.168.1.100 --verbose

# TCP Protocol Buffer Stream (FULL RF traffic capture like Python CLI --listen)
./mesh-debug --host 192.168.1.100 --tcp --verbose

# With packet filtering for focused debugging
./mesh-debug --port COM3 --filter "type:text" --verbose

# Compare connection types:
./mesh-debug --host 192.168.1.100 --verbose      # HTTP (limited)
./mesh-debug --host 192.168.1.100 --tcp --verbose # TCP (full RF traffic)
```

### Development Testing
```bash
# Test against real hardware (requires Meshtastic device)
./mesh-debug.exe --port COM3 --verbose 2>debug.log

# Monitor specific packet types during development
./mesh-debug.exe --port COM3 --filter "type:telemetry,type:position" --verbose

# Performance testing with busy networks
./mesh-debug.exe --port COM3 --verbose | grep "queue full"
```

## Architecture Overview

### Core Design Pattern
The application follows a **layered architecture** with clear separation between connection handling, protocol processing, and user interface:

1. **Connection Layer** (`internal/serial/`, `internal/wifi/`, `internal/tcp/`): Abstracts different connection methods through a common `Connection` interface
2. **Protocol Layer** (`internal/meshtastic/`): Handles Meshtastic packet parsing, statistics, and client management  
3. **Application Layer** (`internal/app/`): Orchestrates components and manages application lifecycle
4. **UI Layer** (`internal/ui/`): Bubble Tea-based TUI with multiple views and real-time updates

### Key Architectural Decisions

**Connection Abstraction**: Serial, WiFi, and TCP connections all implement the same `Connection` interface, allowing the application to work with any transport seamlessly. The WiFi implementation includes fallback mechanisms for legacy firmware (2.6.11) that lacks WebSocket support. The TCP implementation provides full RF traffic capture using protocol buffer streams, equivalent to the Python CLI's `--listen` option.

**Packet Processing Pipeline**: 
- Raw data → `ParseRawPacket()` → Typed packets → Statistics aggregation → UI updates
- Uses Go channels for packet queuing with backpressure handling
- Subscriber pattern for decoupling packet processing from UI updates

**Real-time UI Architecture**:
- Bubble Tea model handles all UI state and events
- Multiple views (Packets, Statistics, Details, Help) with tab navigation
- Background goroutines for packet processing don't block UI rendering
- Automatic reconnection and error recovery built into connection handlers

### Data Flow
```
Meshtastic Device → Connection Layer → Meshtastic Client → Packet Parser
                                           ↓
UI Model ← Packet Subscribers ← Statistics Aggregator ← Processed Packets
```

### Thread Safety Considerations
- All connection implementations use `sync.RWMutex` for thread-safe access
- Packet statistics are protected with mutexes to prevent race conditions
- UI updates happen on the main Bubble Tea event loop to avoid concurrent access to UI state

## Development Patterns

### Adding New Connection Types
1. Implement the `Connection` interface in a new package under `internal/`
2. Add the connection type to `app.ConnectionType` enum
3. Update `initConnection()` in `internal/app/app.go`

### Adding New Packet Types  
1. Add to `PacketType` enum and `PacketTypeNames` map in `internal/meshtastic/packet.go`
2. Create corresponding data structure (e.g., `TelemetryData`)
3. Update `inferPacketType()` and `decodePayload()` functions
4. Add UI handling in `updatePacketTable()` for display formatting

### Debugging Connection Issues
- Use `--verbose` flag to enable detailed logging to stderr
- Connection state is logged at each major transition
- Packet parsing failures are logged with hex dumps
- WebSocket fallback behavior is logged for WiFi connections

### UI Development Guidelines
- All views implement consistent header/footer patterns
- Use `styles.go` for consistent visual styling across views
- Keyboard navigation follows vim-like patterns (hjkl) plus arrow keys
- Views should handle window resize events gracefully

## Platform-Specific Notes

### Windows Development
- Primary platform with full COM port support
- PowerShell scripts handle Go installation and build validation
- Serial port detection via Windows Device Manager
- Executable naming uses `.exe` extension

### Legacy Firmware Support
- WiFi connections gracefully degrade for firmware 2.6.11
- HTTP polling replaces WebSocket streaming when unavailable  
- Command sending has limited support over HTTP in legacy firmware
- Always test against multiple firmware versions when modifying WiFi code

### Hardware Requirements
- Meshtastic device with either USB/serial OR WiFi connectivity
- For development: recommend having both connection types available
- Debug mode should be enabled on device for full packet visibility

## Testing Strategy

### Manual Testing Checklist
- [ ] Serial connection establishment and packet capture
- [ ] WiFi connection with WebSocket support
- [ ] WiFi fallback to HTTP polling for legacy firmware
- [ ] Packet filtering and view navigation
- [ ] Error handling and reconnection scenarios
- [ ] UI responsiveness with high packet rates

### Device Configuration for Testing
```bash
# Enable debug mode for full packet visibility
--set debug_log_enabled true

# Configure WiFi (if using WiFi connection)
--set wifi_ssid "YourNetworkName"
--set wifi_password "YourPassword" 
--set webserver_enabled true
```

## Common Development Tasks

### Adding New Filter Types
1. Extend filter parsing in `internal/filters/filters.go`
2. Update `addPacket()` method in UI model to apply new filters
3. Add filter help text to keyboard shortcuts

### Improving Packet Parsing
- Real Meshtastic packets use Protocol Buffers - current parsing is simplified
- `ParseRawPacket()` in `packet.go` contains parsing heuristics that may need updates
- Consider integrating official Meshtastic protobuf definitions for accuracy

### Performance Optimization
- Packet history is limited to 1000 entries to prevent memory growth
- Consider implementing packet archiving for longer debugging sessions
- UI rendering optimizations may be needed for very high packet rates

### Cross-Platform Porting
- Serial port handling is already cross-platform via `go.bug.st/serial`
- Main porting effort would be installation scripts and path handling
- UI and core application logic should work unchanged on Linux/macOS