# Meshtastic Device Configuration for RF Traffic Capture

## Issue: Only Seeing Heartbeat Telemetry

If you're only seeing heartbeat telemetry packets and not other RF traffic, the issue is likely one of the following:

## 1. Device Configuration

Your Meshtastic device needs to be properly configured to capture and forward all RF traffic. Connect to your device via CLI and run these commands:

### Essential Configuration Commands:
```bash
# Enable debug logging to see all packet activity
--set debug_log_enabled true

# Enable promiscuous mode to capture all RF traffic (if supported)
--set promiscuous_mode true

# Set router mode to capture more network traffic
--set router_enabled true

# Enable relay mode to see relayed packets
--set repeater_enabled true

# Configure logging level for maximum verbosity
--set serial_plugin_echo true
--set serial_plugin_enabled true

# Check current settings
--get debug_log_enabled
--get router_enabled
--get repeater_enabled
```

## 2. Network Activity

RF traffic depends on network activity:

- **Other nodes**: You need other Meshtastic devices on the network sending messages
- **Message types**: Different apps send different packet types:
  - Position updates (GPS apps)
  - Text messages (chat apps) 
  - Node info broadcasts
  - Telemetry data from sensors
  - Range testing packets

## 3. Test RF Traffic Generation

To verify your setup is working, try generating some test traffic:

```bash
# Send a test message (replace with actual node ID)
--sendtext "Test message from debug setup"

# Send to specific node
--dest !12345678 --sendtext "Direct message test"

# Request node info to generate NODE_INFO packets
--dest !12345678 --request-node-info

# Start range test to generate RANGE_TEST packets  
--range-test

# Request position updates
--dest !12345678 --request-position
```

## 4. Check Your Device's Role

Different device roles capture different traffic:

```bash
# Check current role
--get role

# Set to router for maximum packet visibility
--set role ROUTER

# Or set to repeater to relay all traffic
--set role REPEATER
```

## 5. Verify Firmware Version

Newer firmware has better debugging capabilities:

```bash
# Check firmware version
--info

# Ensure you have firmware 2.3.0+ for best debugging support
```

## 6. Monitor Network Health

```bash
# Check for other nodes
--nodes

# Check mesh topology  
--trace-route !12345678

# Monitor channel utilization
--get channel_util_enabled
--set channel_util_enabled true
```

## 7. Application Configuration

If using the go-mesh debug tool, run with maximum verbosity:

```bash
# Serial connection with verbose logging
./mesh-debug.exe --port COM3 --verbose 2>debug.log

# Monitor the debug.log file for detailed packet parsing info
tail -f debug.log
```

## Troubleshooting Steps:

1. **Verify device is in debug mode**: Look for debug output in your terminal
2. **Check for other nodes**: Run `--nodes` to see if other devices are visible
3. **Generate test traffic**: Use the commands above to create known packet types
4. **Monitor with filters**: Use `--filter "type:position,type:text"` to focus on specific traffic
5. **Check logs**: Enable verbose logging to see what's being parsed vs. dropped

## Expected Packet Types:

Once configured correctly, you should see:
- **TELEMETRY**: Battery, voltage, sensor data
- **POSITION**: GPS coordinates and location updates  
- **TEXT**: Chat messages and text communications
- **NODE_INFO**: Device information and capabilities
- **ROUTING**: Network routing and mesh topology
- **ADMIN**: Configuration changes and admin commands
- **RANGE_TEST**: Signal strength testing packets

If you're still only seeing telemetry after these changes, it likely means your local mesh network has very little activity, or other nodes aren't configured to broadcast different packet types.