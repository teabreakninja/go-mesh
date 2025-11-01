# TODO List

## Priority: High

### 🔍 Packet Decoding Improvements
**Status:** Not Started  
**Description:** Improve protobuf parsing to properly decode and identify different Meshtastic message types instead of showing all packets as "Type=UNKNOWN"

**Current Issue:**
- All packets from TCP stream are being parsed but show as "Type=UNKNOWN"
- FromRadio protobuf messages need better field interpretation
- Different field types (3, 4, 5, 9, 10, 13, 17) are detected but not properly decoded

**Requirements:**
- Enhance protobuf parsing in the stream reader
- Map protobuf field numbers to proper Meshtastic message types
- Implement proper message type detection (NodeInfo, Position, Text, etc.)
- Display meaningful packet information instead of raw hex dumps

**Files to Modify:**
- Stream reader packet parsing logic
- Protobuf message type definitions
- UI display formatting for different packet types

**Expected Outcome:**
- Packets should show proper types like "TEXT", "POSITION", "NODEINFO", etc.
- Meaningful packet content display in the TUI
- Better filtering and analysis capabilities

---

## Priority: Medium

### 📱 UI Enhancements
- Improve TUI layout and packet filtering options
- Add packet statistics and metrics display
- Implement search and filtering by message content

### 🔧 Performance Optimizations
- Optimize packet processing for high-volume streams
- Add packet buffering and rate limiting options
- Memory usage improvements for long-running sessions

---

## Priority: Low

### 📝 Documentation
- Add detailed packet type reference
- Create troubleshooting guide
- Add more usage examples

---

*Last Updated: 2025-09-22*
