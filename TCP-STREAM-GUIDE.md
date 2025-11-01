# TCP Protocol Buffer Stream Guide

## Overview

The go-mesh debugger now supports TCP protocol buffer streaming, which provides **full RF traffic capture** exactly like the Meshtastic Python CLI's `--listen` option. This gives you access to ALL packet types and RF activity, not just heartbeat telemetry.

## Why Use TCP Stream Instead of HTTP/WiFi?

| Connection Type | Packet Visibility | Real-time | Use Case |
|----------------|-------------------|-----------|----------|
| **Serial** | Full RF traffic | Yes | Direct USB connection |
| **HTTP/WiFi** | Limited (device status only) | No (polling) | Legacy firmware, basic monitoring |
| **TCP Stream** | **Full RF traffic** | Yes | Network-based full packet capture |

## How to Use TCP Protocol Buffer Stream

### 1. Configure Your Meshtastic Device

First, enable the TCP API on your Meshtastic device:

```bash
# Connect to your device via serial or Meshtastic app
meshtastic --set network.wifi_enabled true
meshtastic --set network.wifi_ssid "YourNetwork"
meshtastic --set network.wifi_password "YourPassword"

# Enable TCP API (this is the key setting!)
meshtastic --set network.api_enabled true

# Optional: Enable more detailed logging
meshtastic --set device.debug_log_enabled true
meshtastic --set device.serial_enabled true

# Reboot the device to apply settings
meshtastic --reboot
```

### 2. Find Your Device's IP Address

```bash
# Check your device info to get its IP
meshtastic --info

# Or check your router's DHCP clients
# Or use network scanning tools
```

### 3. Connect Using TCP Protocol Buffer Stream

```bash
# Connect via TCP for full RF traffic capture
./mesh-debug --host 192.168.1.100 --tcp --verbose

# The default TCP port is 4403 (Meshtastic protocol buffer port)
./mesh-debug --host 192.168.1.100 --tcp --tcp-port 4403 --verbose

# With packet filtering
./mesh-debug --host 192.168.1.100 --tcp --filter "type:text,type:position" --verbose
```

### 4. Compare Connection Types

```bash
# HTTP/WiFi (limited visibility)
./mesh-debug --host 192.168.1.100 --verbose

# TCP Protocol Buffer Stream (full RF traffic)
./mesh-debug --host 192.168.1.100 --tcp --verbose

# Serial (full RF traffic, direct connection)
./mesh-debug --port /dev/ttyUSB0 --verbose  # Linux/macOS
./mesh-debug --port COM3 --verbose          # Windows
```

## What You'll See with TCP Stream

Once connected via TCP protocol buffer stream, you should see:

### All Packet Types:
- **TEXT**: Chat messages and text communications
- **POSITION**: GPS coordinates and location updates
- **TELEMETRY**: Battery, voltage, sensor data (not just heartbeats!)
- **NODE_INFO**: Device information and node announcements  
- **ROUTING**: Network routing and mesh topology packets
- **ADMIN**: Configuration changes and admin commands
- **RANGE_TEST**: Signal strength testing packets

### Real RF Activity:
- Packets from other nodes in the mesh network
- Relayed/forwarded packets with hop counts
- Broadcast vs. directed messages
- Signal strength (RSSI/SNR) data
- Channel utilization information

### Example Output:
```
Connecting via TCP protocol buffer stream to 192.168.1.100:4403
This will capture all RF traffic like the Python CLI --listen option
Starting Meshtastic debugger: Connected to 192.168.1.100:4403 via TCP (Protocol Buffer Stream)

Time     From      To        Type      Channel Hops RSSI Data
10:30:15 !a1b2c3d4 !broadcast TEXT     0       0/3  -85  Hello mesh network!
10:30:16 !e5f6g7h8 !a1b2c3d4  POSITION 0       1/3  -92  Lat:37.7749 Lon:-122.4194
10:30:17 !x9y8z7w6 !broadcast TELEMETRY 0      2/3  -78  Batt:87% V:4.12
10:30:18 !a1b2c3d4 !e5f6g7h8  TEXT     0       0/3  -85  Thanks for the position update
```

## Troubleshooting TCP Connection

### Connection Failed
```bash
# Check if TCP API is enabled on device
meshtastic --get network.api_enabled  # Should be true

# Verify device is connected to WiFi
meshtastic --get network.wifi_enabled  # Should be true

# Test basic connectivity
ping 192.168.1.100

# Check if port 4403 is open
telnet 192.168.1.100 4403
# or
nc -zv 192.168.1.100 4403
```

### No Packets Received
1. **Generate test traffic**: Send messages from another device or app
2. **Check network activity**: Ensure other mesh nodes are active
3. **Verify debug logging**: Enable verbose mode with `--verbose`
4. **Test with serial first**: Compare results with direct serial connection

### "TCP Stream Not Working"
```bash
# Fall back to serial connection to verify the device works
./mesh-debug --port /dev/ttyUSB0 --verbose

# Or try HTTP mode first
./mesh-debug --host 192.168.1.100 --verbose
```

## Advantages of TCP Protocol Buffer Stream

### ✅ Full RF Traffic Capture
- See all packet types, not just device status
- Same level of visibility as Python CLI `--listen`
- Real-time streaming, no polling delays

### ✅ Network-Based Connection
- No need for physical USB/serial connection
- Can monitor remote devices over WiFi
- Multiple clients can connect simultaneously

### ✅ Protocol Buffer Efficiency
- Direct binary protocol (not JSON over HTTP)
- Lower latency and overhead
- More reliable packet framing

## Comparison with Python CLI

The TCP stream implementation provides equivalent functionality to:

```bash
# Python Meshtastic CLI
meshtastic --host 192.168.1.100 --listen

# go-mesh TCP equivalent
./mesh-debug --host 192.168.1.100 --tcp --verbose
```

Both will give you complete RF traffic visibility and real-time packet streaming.

## Next Steps

1. **Try the TCP connection** with your Meshtastic device
2. **Generate test traffic** by sending messages between devices
3. **Compare results** with serial and HTTP connections to see the difference
4. **Use packet filtering** to focus on specific traffic types
5. **Monitor network activity** during peak usage times

The TCP protocol buffer stream should finally give you the full RF traffic visibility you've been looking for!