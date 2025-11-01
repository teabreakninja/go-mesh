package filters

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go-mesh/internal/meshtastic"
)

// Filter represents a packet filter
type Filter interface {
	Match(*meshtastic.Packet) bool
	String() string
}

// FilterSet holds multiple filters
type FilterSet struct {
	filters []Filter
	mode    FilterMode
}

// FilterMode determines how multiple filters are combined
type FilterMode int

const (
	ModeAND FilterMode = iota // All filters must match
	ModeOR                    // Any filter can match
)

// NewFilterSet creates a new filter set
func NewFilterSet(mode FilterMode) *FilterSet {
	return &FilterSet{
		filters: make([]Filter, 0),
		mode:    mode,
	}
}

// Add adds a filter to the set
func (fs *FilterSet) Add(filter Filter) {
	fs.filters = append(fs.filters, filter)
}

// Match checks if a packet matches the filter set
func (fs *FilterSet) Match(packet *meshtastic.Packet) bool {
	if len(fs.filters) == 0 {
		return true // No filters means all packets match
	}

	switch fs.mode {
	case ModeAND:
		for _, filter := range fs.filters {
			if !filter.Match(packet) {
				return false
			}
		}
		return true

	case ModeOR:
		for _, filter := range fs.filters {
			if filter.Match(packet) {
				return true
			}
		}
		return false

	default:
		return true
	}
}

// String returns a string representation of the filter set
func (fs *FilterSet) String() string {
	if len(fs.filters) == 0 {
		return "No filters"
	}

	var parts []string
	for _, filter := range fs.filters {
		parts = append(parts, filter.String())
	}

	separator := " AND "
	if fs.mode == ModeOR {
		separator = " OR "
	}

	return strings.Join(parts, separator)
}

// Specific filter implementations

// NodeFilter filters packets by sender or receiver node ID
type NodeFilter struct {
	nodeID uint32
	field  string // "from", "to", "any"
}

func NewNodeFilter(nodeID uint32, field string) *NodeFilter {
	return &NodeFilter{nodeID: nodeID, field: field}
}

func (f *NodeFilter) Match(packet *meshtastic.Packet) bool {
	switch f.field {
	case "from":
		return packet.From == f.nodeID
	case "to":
		return packet.To == f.nodeID
	case "any":
		return packet.From == f.nodeID || packet.To == f.nodeID
	default:
		return false
	}
}

func (f *NodeFilter) String() string {
	return fmt.Sprintf("Node %s !%08x", f.field, f.nodeID)
}

// TypeFilter filters packets by type
type TypeFilter struct {
	packetType meshtastic.PacketType
}

func NewTypeFilter(packetType meshtastic.PacketType) *TypeFilter {
	return &TypeFilter{packetType: packetType}
}

func (f *TypeFilter) Match(packet *meshtastic.Packet) bool {
	return packet.Type == f.packetType
}

func (f *TypeFilter) String() string {
	return fmt.Sprintf("Type %s", meshtastic.PacketTypeNames[f.packetType])
}

// ChannelFilter filters packets by channel
type ChannelFilter struct {
	channel uint8
}

func NewChannelFilter(channel uint8) *ChannelFilter {
	return &ChannelFilter{channel: channel}
}

func (f *ChannelFilter) Match(packet *meshtastic.Packet) bool {
	return packet.Channel == f.channel
}

func (f *ChannelFilter) String() string {
	return fmt.Sprintf("Channel %d", f.channel)
}

// HopFilter filters packets by hop count
type HopFilter struct {
	minHops int
	maxHops int
}

func NewHopFilter(minHops, maxHops int) *HopFilter {
	return &HopFilter{minHops: minHops, maxHops: maxHops}
}

func (f *HopFilter) Match(packet *meshtastic.Packet) bool {
	hops := int(packet.HopCount)
	return hops >= f.minHops && hops <= f.maxHops
}

func (f *HopFilter) String() string {
	if f.minHops == f.maxHops {
		return fmt.Sprintf("Hops %d", f.minHops)
	}
	return fmt.Sprintf("Hops %d-%d", f.minHops, f.maxHops)
}

// SignalFilter filters packets by signal strength
type SignalFilter struct {
	minRSSI int32
	maxRSSI int32
}

func NewSignalFilter(minRSSI, maxRSSI int32) *SignalFilter {
	return &SignalFilter{minRSSI: minRSSI, maxRSSI: maxRSSI}
}

func (f *SignalFilter) Match(packet *meshtastic.Packet) bool {
	if packet.RxRSSI == 0 {
		return false // No signal data
	}
	return packet.RxRSSI >= f.minRSSI && packet.RxRSSI <= f.maxRSSI
}

func (f *SignalFilter) String() string {
	return fmt.Sprintf("RSSI %d-%d dBm", f.minRSSI, f.maxRSSI)
}

// TimeFilter filters packets by time range
type TimeFilter struct {
	start time.Time
	end   time.Time
}

func NewTimeFilter(start, end time.Time) *TimeFilter {
	return &TimeFilter{start: start, end: end}
}

func (f *TimeFilter) Match(packet *meshtastic.Packet) bool {
	return packet.RxTime.After(f.start) && packet.RxTime.Before(f.end)
}

func (f *TimeFilter) String() string {
	return fmt.Sprintf("Time %s-%s", f.start.Format("15:04:05"), f.end.Format("15:04:05"))
}

// TextFilter filters packets containing specific text
type TextFilter struct {
	pattern *regexp.Regexp
	field   string // "text", "any"
}

func NewTextFilter(pattern string, field string) (*TextFilter, error) {
	regex, err := regexp.Compile("(?i)" + pattern) // Case insensitive
	if err != nil {
		return nil, err
	}
	return &TextFilter{pattern: regex, field: field}, nil
}

func (f *TextFilter) Match(packet *meshtastic.Packet) bool {
	switch f.field {
	case "text":
		if textData, ok := packet.DecodedData.(*meshtastic.TextData); ok {
			return f.pattern.MatchString(textData.Text)
		}
		return false
	case "any":
		// Check all text fields
		if textData, ok := packet.DecodedData.(*meshtastic.TextData); ok {
			if f.pattern.MatchString(textData.Text) {
				return true
			}
		}
		// Check raw data as string
		return f.pattern.Match(packet.Raw)
	default:
		return false
	}
}

func (f *TextFilter) String() string {
	return fmt.Sprintf("Text /%s/", f.pattern.String())
}

// ParseFilterExpression parses a filter expression string
func ParseFilterExpression(expr string) (*FilterSet, error) {
	filterSet := NewFilterSet(ModeAND)
	
	if expr == "" {
		return filterSet, nil
	}

	// Split by common delimiters
	parts := strings.FieldsFunc(expr, func(c rune) bool {
		return c == ',' || c == ';' || c == ' '
	})

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		filter, err := parseFilterPart(part)
		if err != nil {
			return nil, fmt.Errorf("invalid filter '%s': %w", part, err)
		}

		if filter != nil {
			filterSet.Add(filter)
		}
	}

	return filterSet, nil
}

// parseFilterPart parses a single filter part
func parseFilterPart(part string) (Filter, error) {
	// Node ID filter: from:!12345678 or to:!12345678 or node:!12345678
	if strings.HasPrefix(part, "from:!") {
		nodeStr := strings.TrimPrefix(part, "from:!")
		if nodeID, err := strconv.ParseUint(nodeStr, 16, 32); err == nil {
			return NewNodeFilter(uint32(nodeID), "from"), nil
		}
	}
	if strings.HasPrefix(part, "to:!") {
		nodeStr := strings.TrimPrefix(part, "to:!")
		if nodeID, err := strconv.ParseUint(nodeStr, 16, 32); err == nil {
			return NewNodeFilter(uint32(nodeID), "to"), nil
		}
	}
	if strings.HasPrefix(part, "node:!") {
		nodeStr := strings.TrimPrefix(part, "node:!")
		if nodeID, err := strconv.ParseUint(nodeStr, 16, 32); err == nil {
			return NewNodeFilter(uint32(nodeID), "any"), nil
		}
	}

	// Type filter: type:text or type:position
	if strings.HasPrefix(part, "type:") {
		typeStr := strings.TrimPrefix(part, "type:")
		for packetType, name := range meshtastic.PacketTypeNames {
			if strings.EqualFold(typeStr, name) {
				return NewTypeFilter(packetType), nil
			}
		}
	}

	// Channel filter: channel:0
	if strings.HasPrefix(part, "channel:") {
		channelStr := strings.TrimPrefix(part, "channel:")
		if channel, err := strconv.ParseUint(channelStr, 10, 8); err == nil {
			return NewChannelFilter(uint8(channel)), nil
		}
	}

	// Hop filter: hops:2 or hops:1-3
	if strings.HasPrefix(part, "hops:") {
		hopStr := strings.TrimPrefix(part, "hops:")
		if strings.Contains(hopStr, "-") {
			parts := strings.Split(hopStr, "-")
			if len(parts) == 2 {
				if min, err1 := strconv.Atoi(parts[0]); err1 == nil {
					if max, err2 := strconv.Atoi(parts[1]); err2 == nil {
						return NewHopFilter(min, max), nil
					}
				}
			}
		} else {
			if hops, err := strconv.Atoi(hopStr); err == nil {
				return NewHopFilter(hops, hops), nil
			}
		}
	}

	// Signal filter: rssi:-80 or rssi:-100--80
	if strings.HasPrefix(part, "rssi:") {
		rssiStr := strings.TrimPrefix(part, "rssi:")
		if strings.Contains(rssiStr, "-") && strings.Count(rssiStr, "-") > 1 {
			// Handle negative numbers in range: -100--80
			parts := strings.Split(rssiStr, "-")
			if len(parts) >= 3 {
				minStr := "-" + parts[len(parts)-2]
				maxStr := "-" + parts[len(parts)-1]
				if min, err1 := strconv.ParseInt(minStr, 10, 32); err1 == nil {
					if max, err2 := strconv.ParseInt(maxStr, 10, 32); err2 == nil {
						return NewSignalFilter(int32(min), int32(max)), nil
					}
				}
			}
		} else {
			if rssi, err := strconv.ParseInt(rssiStr, 10, 32); err == nil {
				return NewSignalFilter(int32(rssi), 0), nil // Single value filter
			}
		}
	}

	// Text filter: text:"hello" or text:hello
	if strings.HasPrefix(part, "text:") {
		textStr := strings.TrimPrefix(part, "text:")
		textStr = strings.Trim(textStr, "\"'")
		return NewTextFilter(textStr, "text")
	}

	return nil, fmt.Errorf("unknown filter format")
}

// Analysis functions

// AnalyzePackets performs analysis on a slice of packets
func AnalyzePackets(packets []*meshtastic.Packet) *PacketAnalysis {
	if len(packets) == 0 {
		return &PacketAnalysis{}
	}

	analysis := &PacketAnalysis{
		TotalPackets:     len(packets),
		TypeDistribution: make(map[meshtastic.PacketType]int),
		NodeActivity:     make(map[uint32]int),
		ChannelActivity:  make(map[uint8]int),
		HopDistribution:  make(map[uint8]int),
		TimeRange:        TimeRange{Start: packets[0].RxTime, End: packets[0].RxTime},
	}

	var rssiSum, snrSum float64
	var rssiCount, snrCount int

	for _, packet := range packets {
		// Type distribution
		analysis.TypeDistribution[packet.Type]++

		// Node activity
		analysis.NodeActivity[packet.From]++
		if packet.To != 0xFFFFFFFF {
			analysis.NodeActivity[packet.To]++
		}

		// Channel activity
		analysis.ChannelActivity[packet.Channel]++

		// Hop distribution
		analysis.HopDistribution[packet.HopCount]++

		// Time range
		if packet.RxTime.Before(analysis.TimeRange.Start) {
			analysis.TimeRange.Start = packet.RxTime
		}
		if packet.RxTime.After(analysis.TimeRange.End) {
			analysis.TimeRange.End = packet.RxTime
		}

		// Signal statistics
		if packet.RxRSSI != 0 {
			rssiSum += float64(packet.RxRSSI)
			rssiCount++
			if analysis.SignalStats.MinRSSI == 0 || packet.RxRSSI < analysis.SignalStats.MinRSSI {
				analysis.SignalStats.MinRSSI = packet.RxRSSI
			}
			if packet.RxRSSI > analysis.SignalStats.MaxRSSI {
				analysis.SignalStats.MaxRSSI = packet.RxRSSI
			}
		}

		if packet.RxSNR != 0 {
			snrSum += float64(packet.RxSNR)
			snrCount++
			if analysis.SignalStats.MinSNR == 0 || packet.RxSNR < analysis.SignalStats.MinSNR {
				analysis.SignalStats.MinSNR = packet.RxSNR
			}
			if packet.RxSNR > analysis.SignalStats.MaxSNR {
				analysis.SignalStats.MaxSNR = packet.RxSNR
			}
		}
	}

	// Calculate averages
	if rssiCount > 0 {
		analysis.SignalStats.AvgRSSI = float32(rssiSum / float64(rssiCount))
	}
	if snrCount > 0 {
		analysis.SignalStats.AvgSNR = float32(snrSum / float64(snrCount))
	}

	return analysis
}

// PacketAnalysis holds analysis results
type PacketAnalysis struct {
	TotalPackets     int                               `json:"total_packets"`
	TypeDistribution map[meshtastic.PacketType]int     `json:"type_distribution"`
	NodeActivity     map[uint32]int                    `json:"node_activity"`
	ChannelActivity  map[uint8]int                     `json:"channel_activity"`
	HopDistribution  map[uint8]int                     `json:"hop_distribution"`
	SignalStats      SignalStatistics                  `json:"signal_stats"`
	TimeRange        TimeRange                         `json:"time_range"`
}

// SignalStatistics holds signal strength statistics
type SignalStatistics struct {
	MinRSSI int32   `json:"min_rssi"`
	MaxRSSI int32   `json:"max_rssi"`
	AvgRSSI float32 `json:"avg_rssi"`
	MinSNR  float32 `json:"min_snr"`
	MaxSNR  float32 `json:"max_snr"`
	AvgSNR  float32 `json:"avg_snr"`
}

// TimeRange holds time range information
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Duration returns the duration of the time range
func (tr *TimeRange) Duration() time.Duration {
	return tr.End.Sub(tr.Start)
}
