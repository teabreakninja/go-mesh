# Meshtastic Terminal Debugger Usage Guide

## Quick Start

1. **Install Go** (if not already installed):
   ```powershell
   # Run the installation script
   .\install-go.ps1
   ```

2. **Build the application**:
   ```bash
   go build -o mesh-debug.exe ./cmd/mesh-debug
   ```

3. **Connect your Meshtastic device** (choose one method):
   
   **Option A: Serial/USB Connection**
   - Open Device Manager in Windows
   - Look for "USB Serial Device" or similar under "Ports (COM & LPT)"
   - Note the COM port number (e.g., COM3, COM4)
   
   **Option B: WiFi Connection**
   - Ensure your Meshtastic device is connected to WiFi
   - Find the device's IP address (check your router or use the Meshtastic app)
   - Ensure the web interface is enabled on the device

4. **Run the debugger**:
   ```bash
   # Serial connection
   .\mesh-debug.exe --port COM3
   
   # WiFi connection
   .\mesh-debug.exe --host 192.168.1.100
   ```

## Command Line Options

```bash
mesh-debug [flags]

Connection Flags (choose one):
  -p, --port string     Serial port of Meshtastic device (e.g., COM3)
      --host string     IP address or hostname of Meshtastic device (e.g., 192.168.1.100)

Serial Options:
  -b, --baud int        Baud rate for serial connection (default 115200)

WiFi Options:
      --tcp-port int    HTTP port for WiFi connection (default 80)

Common Options:
  -v, --verbose         Enable verbose logging
  -f, --filter string   Filter packets (node ID, message type, etc.)
  -h, --help           Help for mesh-debug
```

## Examples

### Basic Usage

```bash
# Serial connection
.\mesh-debug.exe --port COM3

# WiFi connection
.\mesh-debug.exe --host 192.168.1.100

# Serial with verbose logging
.\mesh-debug.exe --port COM3 --verbose

# WiFi with custom port
.\mesh-debug.exe --host 192.168.1.100 --tcp-port 8080

# Serial with custom baud rate
.\mesh-debug.exe --port COM4 --baud 9600
```

### Filtering Packets

```bash
# Filter by message type (serial)
.\mesh-debug.exe --port COM3 --filter "type:text"

# Filter by message type (WiFi)
.\mesh-debug.exe --host 192.168.1.100 --filter "type:text"

# Filter by node ID
.\mesh-debug.exe --host 192.168.1.100 --filter "from:!12345678"

# Filter by channel
.\mesh-debug.exe --port COM3 --filter "channel:0"

# Multiple filters (AND logic)
.\mesh-debug.exe --host 192.168.1.100 --filter "type:text,channel:0"
```

## Interface Navigation

### Main View - Packet List

The default view shows a real-time list of packets with the following columns:

- **Time**: When the packet was received
- **From**: Sender node ID (abbreviated)
- **To**: Destination node ID (abbreviated)
- **Type**: Packet type (TEXT, POSITION, TELEMETRY, etc.)
- **Channel**: Channel number
- **Hops**: Current hops / hop limit
- **RSSI**: Received signal strength
- **Data**: Preview of packet content

### Keyboard Controls

- **↑/↓ or k/j**: Navigate up/down in packet list
- **Enter**: View detailed packet information
- **Tab**: Switch between views (Packets → Statistics → Details → Help)
- **?**: Toggle help view
- **f**: Toggle packet filtering (when available)
- **c**: Clear packet list
- **r**: Refresh display
- **q, Esc, Ctrl+C**: Quit application

### Views

1. **Packets View**: Real-time packet list (default)
2. **Statistics View**: Network statistics and analysis
3. **Details View**: Detailed information about selected packet
4. **Help View**: Keyboard shortcuts and usage information

## Filter Syntax

### Node ID Filters
```
from:!12345678    # Packets from specific node
to:!87654321      # Packets to specific node
node:!12345678    # Packets from or to specific node
```

### Type Filters
```
type:text         # Text messages
type:position     # Position/GPS data
type:telemetry    # Device telemetry
type:nodeinfo     # Node information
type:routing      # Routing packets
```

### Other Filters
```
channel:0         # Specific channel
hops:2            # Exact hop count
hops:1-3          # Hop count range
rssi:-80          # Minimum RSSI
rssi:-100--80     # RSSI range
text:hello        # Text containing "hello"
```

### Combining Filters
```
# Multiple filters with AND logic
type:text,channel:0,from:!12345678

# Use spaces or commas as separators
type:text channel:0 from:!12345678
```

## Troubleshooting

### Common Issues

1. **Serial Connection Issues**
   - **"Port not found" or "Permission denied"**
     - Ensure Meshtastic device is connected via USB
     - Check that no other application is using the serial port
     - Try a different COM port
     - Run as Administrator if needed

2. **WiFi Connection Issues**
   - **"Connection refused" or "No route to host"**
     - Ensure device is connected to WiFi network
     - Verify the IP address is correct
     - Check if web interface is enabled on the device
     - Try pinging the device: `ping 192.168.1.100`
   - **"HTTP 404" or "WebSocket connection failed"**
     - Ensure device firmware supports web interface
     - Try different HTTP port with `--tcp-port` flag
     - Check device web interface in browser first

2. **"Go not found"**
   - Install Go using `.\install-go.ps1`
   - Restart PowerShell after installation
   - Verify Go is in your PATH: `go version`

3. **No packets appearing**
   - Ensure device is in range of mesh network
   - Try enabling verbose mode: `--verbose`
   - Check device configuration and firmware

4. **Build errors**
   - Run `go mod tidy` to download dependencies
   - Ensure you're in the correct directory
   - Check Go version: `go version` (requires Go 1.21+)

### Performance Tips

- Use filters to reduce packet volume and improve performance
- Clear packet history regularly with 'c' key
- Close other applications using the serial port

### Device Configuration

For best results with packet debugging:

**All Connection Types:**
1. **Enable debug mode** on your Meshtastic device:
   ```
   --set debug_log_enabled true
   ```

2. **Set appropriate channel settings** for your use case

3. **Consider hop limits** - higher hop limits show more network activity

**WiFi-Specific Configuration:**
1. **Enable WiFi** on your Meshtastic device:
   ```
   --set wifi_ssid "YourNetworkName"
   --set wifi_password "YourPassword"
   --set wifi_ap_mode false
   ```

2. **Enable web interface** (usually enabled by default):
   ```
   --set webserver_enabled true
   ```

3. **Find device IP address** using the Meshtastic app or check your router's DHCP client list

## Advanced Usage

### Packet Analysis

The Statistics view provides detailed analysis:
- Total packet counts by type and channel
- Signal strength statistics (RSSI/SNR)
- Network activity over time
- Node participation metrics

### Raw Packet Inspection

In the Details view, you can examine:
- Complete packet headers
- Raw binary data (hex dump)
- Decoded payload information
- Signal strength and timing data

### Logging

Enable verbose logging to see detailed debug information:
```bash
.\mesh-debug.exe --port COM3 --verbose 2>debug.log
```

This saves debug information to `debug.log` while keeping the UI clean.

## Integration with Other Tools

The packet data can be used with other analysis tools:
- Export functionality (planned feature)
- JSON output for automated processing
- Integration with network analysis tools

## Known Limitations

- Currently supports Windows (COM ports)
- Limited to serial connection (no TCP/IP or Bluetooth)
- Packet parsing is based on reverse engineering (not official protocol buffers)
- Some advanced packet types may not be fully decoded

## Contributing

To contribute to this project:
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test with real hardware
5. Submit a pull request

For bug reports and feature requests, please provide:
- Device model and firmware version
- Operating system details
- Steps to reproduce the issue
- Log output with `--verbose` flag
