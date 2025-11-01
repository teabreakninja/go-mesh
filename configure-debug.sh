#!/bin/bash

# Meshtastic Debug Configuration Script
# This script enables debug mode and packet capture on your Meshtastic device

echo "🔧 Configuring Meshtastic device for packet debugging..."

# Check if device path/host is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 <device_path_or_host>"
    echo "Examples:"
    echo "  $0 /dev/cu.usbserial-0001     # For USB connected device"
    echo "  $0 192.168.1.100              # For WiFi connected device"
    exit 1
fi

DEVICE="$1"

# Function to send command via serial
send_serial_command() {
    local cmd="$1"
    echo "📡 Sending: $cmd"
    if command -v meshtastic &> /dev/null; then
        meshtastic --port "$DEVICE" $cmd
    else
        echo "⚠️  meshtastic CLI not found. Please install it with:"
        echo "   pip install meshtastic"
        echo "   Or send this command manually: $cmd"
    fi
}

# Function to send command via WiFi
send_wifi_command() {
    local cmd="$1"
    echo "📡 Sending via HTTP: $cmd"
    # This would need to be implemented based on the specific WiFi API
    echo "⚠️  WiFi command sending not yet implemented"
    echo "   Please use the web interface or Meshtastic app to send: $cmd"
}

# Determine connection type
if [[ "$DEVICE" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "🌐 Detected WiFi connection to $DEVICE"
    CONNECTION_TYPE="wifi"
else
    echo "🔌 Detected serial connection to $DEVICE"
    CONNECTION_TYPE="serial"
fi

echo ""
echo "🎯 Applying debug configuration..."

# Enable debug logging
echo "1. Enabling debug logging..."
if [ "$CONNECTION_TYPE" = "serial" ]; then
    send_serial_command "--set debug_log_enabled true"
else
    send_wifi_command "--set debug_log_enabled true"
fi

# Set log level to debug
echo "2. Setting log level to debug..."
if [ "$CONNECTION_TYPE" = "serial" ]; then
    send_serial_command "--set serial_log_level 10"
else
    send_wifi_command "--set serial_log_level 10"
fi

# Enable packet logging (if supported)
echo "3. Configuring for packet capture..."
if [ "$CONNECTION_TYPE" = "serial" ]; then
    send_serial_command "--set is_router false"  # Ensure not in router mode
    send_serial_command "--set is_low_power false"  # Disable power saving
else
    send_wifi_command "--set is_router false"
    send_wifi_command "--set is_low_power false"
fi

echo ""
echo "✅ Configuration complete!"
echo ""
echo "📋 Next steps:"
echo "1. Wait 10-15 seconds for settings to take effect"
echo "2. Run your mesh-debug application:"
if [ "$CONNECTION_TYPE" = "serial" ]; then
    echo "   ./mesh-debug --port $DEVICE --verbose"
else
    echo "   ./mesh-debug --host $DEVICE --verbose"
fi
echo "3. Look for radio packets from other nearby Meshtastic nodes"
echo ""
echo "🔍 If you still only see telemetry:"
echo "• Make sure other Meshtastic devices are nearby and active"
echo "• Check that your device's firmware supports debug logging"
echo "• Verify mesh network activity (send a message from another node)"
echo ""
echo "📡 To test with actual traffic:"
echo "• Use another Meshtastic device or the mobile app"
echo "• Send a message to 'everyone' or to a specific node"
echo "• You should see both TX and RX packet activity"