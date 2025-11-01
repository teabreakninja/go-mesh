# Meshtastic WiFi Connection Guide

This guide covers connecting to your Meshtastic device via WiFi instead of serial/USB.

## Prerequisites

1. **Meshtastic device with WiFi capability**
   - Most recent Meshtastic devices support WiFi
   - Device must have WiFi antenna connected

2. **Device connected to your network**
   - Device configured with your WiFi network credentials
   - Device must be on the same network as your computer

3. **Web interface enabled**
   - Usually enabled by default on recent firmware
   - Can be verified through the Meshtastic app

## Setting Up WiFi Connection

### Step 1: Configure Device WiFi

Using the Meshtastic CLI or app:

```bash
# Connect device via USB first, then configure WiFi
meshtastic --set wifi_ssid "YourNetworkName"
meshtastic --set wifi_password "YourWiFiPassword"
meshtastic --set wifi_ap_mode false
```

Or using the mobile app:
1. Open Meshtastic app
2. Go to Settings → Radio Configuration → WiFi
3. Enter your network details
4. Save configuration

### Step 2: Find Device IP Address

**Method 1: Check your router**
- Access your router's admin interface
- Look for "DHCP clients" or "Connected devices"
- Find device with name starting with "Meshtastic"

**Method 2: Use Meshtastic app**
- Open app while connected to device
- Go to device info - IP address may be displayed

**Method 3: Network scan**
```bash
# Scan your network (replace with your subnet)
nmap -sn 192.168.1.0/24
```

### Step 3: Test Web Interface

Before using the debugger, test the web interface:

1. Open browser and navigate to `http://DEVICE_IP`
2. You should see the Meshtastic web interface
3. Verify you can see device information and messages

## Using WiFi with mesh-debug

### Basic Connection

```bash
# Connect to device via WiFi
.\mesh-debug.exe --host 192.168.1.100
```

### Advanced Options

```bash
# Specify custom HTTP port
.\mesh-debug.exe --host 192.168.1.100 --tcp-port 8080

# Enable verbose logging for troubleshooting
.\mesh-debug.exe --host 192.168.1.100 --verbose

# Use with packet filtering
.\mesh-debug.exe --host 192.168.1.100 --filter "type:text"
```

## WiFi vs Serial Comparison

| Feature | Serial/USB | WiFi |
|---------|------------|------|
| **Connection** | Direct USB cable | Network connection |
| **Setup** | Plug and play | Requires WiFi configuration |
| **Portability** | Limited by cable | Works anywhere on network |
| **Reliability** | Very stable | Depends on WiFi quality |
| **Performance** | Low latency | Slightly higher latency |
| **Power** | Powers device | Device needs separate power |
| **Security** | Physically secure | Network-dependent security |

## Troubleshooting WiFi Connection

### Common Issues and Solutions

#### Connection Refused
```
Error: failed to initialize connection: failed to connect via HTTP: failed to reach device: Get "http://192.168.1.100:80/api/v1/status": dial tcp 192.168.1.100:80: connectex: No connection could be made because the target machine actively refused it.
```

**Solutions:**
1. **Verify IP address**: Double-check the device IP
2. **Check device power**: Ensure device is powered on
3. **Verify network**: Make sure device is connected to WiFi
4. **Test with browser**: Try accessing `http://DEVICE_IP` in browser
5. **Check firewall**: Temporarily disable Windows Firewall

#### WebSocket Connection Failed
```
Error: failed to connect WebSocket: websocket: bad handshake
```

**Solutions:**
1. **Update firmware**: Ensure device has recent firmware with WebSocket support
2. **Check port**: Try different HTTP port with `--tcp-port` flag
3. **Restart device**: Power cycle the Meshtastic device
4. **Network issues**: Check for proxy or network restrictions

#### HTTP 404 Errors
```
Error: HTTP request failed with status 404
```

**Solutions:**
1. **Firmware compatibility**: Ensure firmware supports web API
2. **API endpoints**: Older firmware may have different API paths
3. **Web interface disabled**: Check if web interface is enabled

#### No Packets Received
Device connects but no packets appear:

**Solutions:**
1. **Check mesh activity**: Ensure there's mesh traffic to capture
2. **Verify WebSocket**: Look for WebSocket connection errors in verbose mode
3. **Firewall blocking**: Check if WebSocket traffic is blocked
4. **Device configuration**: Ensure device is properly configured for mesh

### Network Diagnostics

#### Test Basic Connectivity
```bash
# Ping the device
ping 192.168.1.100

# Test HTTP port
telnet 192.168.1.100 80
```

#### Check Network Configuration
```bash
# Show your network configuration
ipconfig

# Find devices on network
arp -a
```

#### Verify WiFi Settings on Device
Using serial connection:
```bash
# Check current WiFi status
meshtastic --info

# Show WiFi configuration
meshtastic --get wifi_ssid
meshtastic --get wifi_password
```

## Advanced Configuration

### Custom HTTP Port
If default port 80 is blocked or used by another service:

**On device:**
```bash
meshtastic --set webserver_port 8080
```

**In debugger:**
```bash
.\mesh-debug.exe --host 192.168.1.100 --tcp-port 8080
```

### HTTPS Support
Some devices support HTTPS (port 443):

```bash
# Note: HTTPS not currently supported in debugger
# Use HTTP (port 80) for now
```

### Access Point Mode
Device can create its own WiFi network:

**Enable AP mode:**
```bash
meshtastic --set wifi_ap_mode true
meshtastic --set wifi_ssid "Meshtastic-XXXX"
meshtastic --set wifi_password "meshtastic"
```

**Connect to device AP:**
1. Connect computer to device's WiFi network
2. Device IP will typically be `192.168.4.1`
3. Run: `.\mesh-debug.exe --host 192.168.4.1`

## Security Considerations

### Network Security
- Device web interface has no authentication by default
- Anyone on your network can access the device
- Consider using a separate IoT network

### Firewall Configuration
Windows may block incoming connections. If needed:
1. Open Windows Defender Firewall
2. Allow mesh-debug.exe through firewall
3. Or temporarily disable for testing

### VPN and Remote Access
- WiFi connection works over VPN
- Can debug remote devices if network allows
- Consider security implications of remote access

## Performance Tips

### Optimize WiFi Performance
1. **Strong signal**: Ensure good WiFi signal strength
2. **Dedicated network**: Use 5GHz network if available
3. **Reduce interference**: Minimize other network traffic
4. **Update firmware**: Use latest device firmware

### Reduce Latency
1. **Wired connection**: Connect computer via Ethernet
2. **Close applications**: Reduce network usage from other apps
3. **Quality of Service**: Configure router QoS if available

## Comparison with Official Tools

### vs Meshtastic CLI
- **CLI**: Command-line interface, scriptable
- **WiFi Debug**: Real-time GUI, packet analysis

### vs Meshtastic Web Interface
- **Web**: Browser-based, device control
- **WiFi Debug**: Terminal-based, packet debugging

### vs Mobile Apps
- **Mobile**: User-friendly, on-the-go management
- **WiFi Debug**: Developer-focused, detailed analysis

## Best Practices

1. **Backup configuration** before making changes
2. **Test with serial first** to ensure device works
3. **Use static IP** for consistent connection
4. **Monitor battery** when device is not USB-powered
5. **Keep firmware updated** for best compatibility

## API Reference

The WiFi connection uses these Meshtastic web API endpoints:

- `GET /api/v1/status` - Device status
- `GET /api/v1/nodes` - Node information
- `POST /api/v1/messages` - Send messages
- `WebSocket /api/v1/stream` - Real-time packet stream

For more details, see the official Meshtastic web API documentation.

---

**Happy WiFi debugging!** 📶🔍📡
