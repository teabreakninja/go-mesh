package ui

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go-mesh/internal/meshtastic"
	"go-mesh/internal/utils"
)

// ViewMode represents the current view mode
type ViewMode int

const (
	ViewPackets ViewMode = iota
	ViewStatistics
	ViewDetails
	ViewHelp
)

// Model represents the main UI model
type Model struct {
	// Core components
	client       *meshtastic.Client
	logger       *log.Logger
	filter       string
	
	// UI State
	currentView  ViewMode
	width        int
	height       int
	help         help.Model
	keys         keyMap
	
	// Packet display
	packets      []*meshtastic.Packet
	packetTable  table.Model
	selectedRow  int
	
	// Statistics
	stats        *meshtastic.Statistics
	
	// Filters
	filterActive bool
	filterByType meshtastic.PacketType
	filterByNode uint32
	
	// Packet messaging
	packetChan   chan *meshtastic.Packet
	
	// Styles
	styles       *Styles
}

// keyMap defines keyboard shortcuts
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Help    key.Binding
	Quit    key.Binding
	Enter   key.Binding
	Tab     key.Binding
	Filter  key.Binding
	Clear   key.Binding
	Refresh key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.Tab, k.Filter}
}

// FullHelp returns keybindings for the expanded help view
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Enter, k.Tab, k.Filter, k.Clear},
		{k.Refresh, k.Help, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "move left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "move right"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "view details"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch view"),
	),
	Filter: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "toggle filter"),
	),
	Clear: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "clear packets"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
}

// NewModel creates a new UI model
func NewModel(client *meshtastic.Client, filter string, logger *log.Logger) Model {
	// Create packet table
	columns := []table.Column{
		{Title: "Time", Width: 12},
		{Title: "From", Width: 10},
		{Title: "To", Width: 10},
		{Title: "Type", Width: 12},
		{Title: "Channel", Width: 7},
		{Title: "Hops", Width: 6},
		{Title: "RSSI", Width: 8},
		{Title: "Data", Width: 30},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	model := Model{
		client:      client,
		logger:      logger,
		filter:      filter,
		currentView: ViewPackets,
		help:        help.New(),
		keys:        keys,
		packets:     make([]*meshtastic.Packet, 0),
		packetTable: t,
		packetChan:  make(chan *meshtastic.Packet, 100),
		styles:      NewStyles(),
	}

	// Subscribe to packet updates
	client.SubscribeFunc(model.onPacketReceived)

	return model
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	// Start the Meshtastic client
	if err := m.client.Start(); err != nil {
		m.logger.Printf("Failed to start Meshtastic client: %v", err)
	}

	return tea.Batch(
		tea.EnterAltScreen,
		tickCmd(),
		listenForPacketsCmd(m.packetChan),
	)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		m.updateTableSize()

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Help):
			if m.currentView == ViewHelp {
				m.currentView = ViewPackets
			} else {
				m.currentView = ViewHelp
			}

		case key.Matches(msg, m.keys.Tab):
			m.nextView()

		case key.Matches(msg, m.keys.Clear):
			m.clearPackets()

		case key.Matches(msg, m.keys.Filter):
			m.filterActive = !m.filterActive

		case key.Matches(msg, m.keys.Enter):
			if m.currentView == ViewPackets && len(m.packets) > 0 {
				m.currentView = ViewDetails
			}

		case key.Matches(msg, m.keys.Up, m.keys.Down):
			if m.currentView == ViewPackets {
				m.packetTable, cmd = m.packetTable.Update(msg)
				m.selectedRow = m.packetTable.Cursor()
			}
		}

	case tickMsg:
		m.updateStats()
		return m, tickCmd()

	case packetMsg:
		m.addPacket(msg.Packet)
		cmd = listenForPacketsCmd(m.packetChan) // Continue listening
	}

	return m, cmd
}

// View renders the current view
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	switch m.currentView {
	case ViewPackets:
		return m.renderPacketsView()
	case ViewStatistics:
		return m.renderStatisticsView()
	case ViewDetails:
		return m.renderDetailsView()
	case ViewHelp:
		return m.renderHelpView()
	default:
		return "Unknown view"
	}
}

// renderPacketsView renders the main packets view
func (m Model) renderPacketsView() string {
	var sections []string

	// Header
	connectionInfo := "Disconnected"
	if m.client.IsConnected() {
		connectionInfo = m.client.GetConnectionInfo()
	}

	header := m.styles.Header.Render(
		fmt.Sprintf("Meshtastic Packet Debugger - %s", connectionInfo),
	)
	sections = append(sections, header)

	// Connection status message
	if len(m.packets) == 0 {
		if m.client.IsConnected() {
			statusMsg := "⏳ Connected and listening for packets..."
			if connectionInfo != "" && strings.Contains(connectionInfo, "WiFi") {
				statusMsg += "\n📡 Using HTTP polling (legacy firmware) - packets appear every ~5 seconds"
			} else if strings.Contains(connectionInfo, "TCP") {
				statusMsg += "\n📻 Listening for all RF traffic - packets will appear when mesh activity occurs"
			} else if strings.Contains(connectionInfo, "Serial") {
				statusMsg += "\n🔌 Serial connection active - waiting for data from device"
			}
			statusMsg += "\n\nPress 'q' to quit, '?' for help"
			sections = append(sections, m.styles.Stats.Render(statusMsg))
		} else {
			sections = append(sections, m.styles.Stats.Render("❌ No connection established"))
		}
	}

	// Filter status
	if m.filterActive {
		filterInfo := m.styles.Filter.Render("Filter: ACTIVE")
		sections = append(sections, filterInfo)
	}

	// Packet table
	sections = append(sections, m.styles.Table.Render(m.packetTable.View()))

	// Statistics footer
	stats := m.client.GetStatistics()
	footer := m.styles.Footer.Render(
		fmt.Sprintf("Total Packets: %d | Avg RSSI: %.1f dBm | Avg SNR: %.1f",
			stats.TotalPackets, stats.AverageRSSI, stats.AverageSNR),
	)
	sections = append(sections, footer)

	// Help
	sections = append(sections, m.styles.Help.Render(m.help.ShortHelpView(m.keys.ShortHelp())))

	return m.styles.App.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// renderStatisticsView renders the statistics view
func (m Model) renderStatisticsView() string {
	stats := m.client.GetStatistics()
	
	var sections []string
	
	// Header
	sections = append(sections, m.styles.Header.Render("Packet Statistics"))

	// General stats
	generalStats := fmt.Sprintf(`
Total Packets: %d
Average RSSI: %.1f dBm
Average SNR: %.1f
Uptime: %v
Last Packet: %v
`,
		stats.TotalPackets,
		stats.AverageRSSI,
		stats.AverageSNR,
		time.Since(stats.StartTime).Truncate(time.Second),
		stats.LastPacketTime.Format("15:04:05"),
	)
	sections = append(sections, m.styles.Stats.Render(generalStats))

	// Packets by type
	typeStats := "Packets by Type:\n"
	for packetType, count := range stats.PacketsByType {
		typeStats += fmt.Sprintf("  %s: %d\n", meshtastic.PacketTypeNames[packetType], count)
	}
	sections = append(sections, m.styles.Stats.Render(typeStats))

	// Packets by channel
	channelStats := "Packets by Channel:\n"
	for channel, count := range stats.PacketsByChannel {
		channelStats += fmt.Sprintf("  Channel %d: %d\n", channel, count)
	}
	sections = append(sections, m.styles.Stats.Render(channelStats))
	
	// Packet detection statistics
	detectionStats := "Packet Type Detection:\n"
	detectionStats += meshtastic.GetGlobalPacketStats().GetStatsString()
	sections = append(sections, m.styles.Stats.Render(detectionStats))

	// Help
	sections = append(sections, m.styles.Help.Render(m.help.ShortHelpView(m.keys.ShortHelp())))

	return m.styles.App.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// renderDetailsView renders the packet details view
func (m Model) renderDetailsView() string {
	var sections []string
	
	// Header
	sections = append(sections, m.styles.Header.Render("Packet Details"))

	if m.selectedRow >= 0 && m.selectedRow < len(m.packets) {
		packet := m.packets[m.selectedRow]
		nodeDB := m.client.GetNodeDB()
		
		details := fmt.Sprintf(`
ID: %d
From: %s (%s)
To: %s (%s)
Type: %s
Channel: %d
Hops: %s
Signal: %s
Time: %s

Raw Data (%d bytes):
%x

Decoded Data:
%v
`,
			packet.ID,
			packet.GetFromName(nodeDB), packet.GetFromHex(),
			packet.GetToName(nodeDB), packet.GetToHex(),
			packet.GetTypeName(),
			packet.Channel,
			packet.GetHopInfo(),
			packet.GetSignalStrength(),
			packet.RxTime.Format("15:04:05"),
			len(packet.Raw),
			packet.Raw,
			packet.DecodedData,
		)
		
		sections = append(sections, m.styles.Details.Render(details))
	} else {
		sections = append(sections, m.styles.Details.Render("No packet selected"))
	}

	// Help
	sections = append(sections, m.styles.Help.Render(m.help.ShortHelpView(m.keys.ShortHelp())))

	return m.styles.App.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// renderHelpView renders the help view
func (m Model) renderHelpView() string {
	var sections []string
	
	// Header
	sections = append(sections, m.styles.Header.Render("Help"))

	// Full help
	sections = append(sections, m.styles.Help.Render(m.help.FullHelpView(m.keys.FullHelp())))

	return m.styles.App.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// Helper methods

func (m *Model) nextView() {
	m.currentView = (m.currentView + 1) % 4
}

func (m *Model) updateTableSize() {
	m.packetTable.SetWidth(m.width - 4)
	m.packetTable.SetHeight(m.height - 10)
}

func (m *Model) updateStats() {
	m.stats = m.client.GetStatistics()
}

func (m *Model) clearPackets() {
	m.packets = make([]*meshtastic.Packet, 0)
	m.updatePacketTable()
}

func (m *Model) addPacket(packet *meshtastic.Packet) {
	// Apply filters if active
	if m.filterActive {
		if m.filterByType != 0 && packet.Type != m.filterByType {
			return
		}
		if m.filterByNode != 0 && packet.From != m.filterByNode {
			return
		}
	}

	// Add packet to the beginning of the list
	m.packets = append([]*meshtastic.Packet{packet}, m.packets...)
	
	// Limit to last 1000 packets
	if len(m.packets) > 1000 {
		m.packets = m.packets[:1000]
	}
	
	m.updatePacketTable()
}

func (m *Model) updatePacketTable() {
	var rows []table.Row
	
	for _, packet := range m.packets {
		data := ""
		if packet.DecodedData != nil {
			switch d := packet.DecodedData.(type) {
			case *meshtastic.TextData:
				// Show enhanced information for categorized text
				if d.Category != "general" && d.Category != "" {
					// For device startup info, show category and key details
					data = strings.ToUpper(d.Category)
					if len(d.Details) > 0 {
						// Show the most relevant detail
						for key, value := range d.Details {
							if key == "owner" || key == "user" || key == "name" ||
							   key == "firmware" || key == "version" || key == "channel" {
								data += fmt.Sprintf(": %s", value)
								break
							}
						}
					}
				} else {
					data = d.Text
				}
			case *meshtastic.PositionData:
				lat := meshtastic.GetLatitudeDegrees(d)
				lon := meshtastic.GetLongitudeDegrees(d)
				if lat == 0 && lon == 0 {
					// Check if coordinates are actually missing (nil pointers) or zero
					if d.LatitudeI == nil && d.LongitudeI == nil {
						data = "GPS: No fix (no data)"
					} else {
						data = "GPS: No fix (0,0)"
					}
				} else {
					data = fmt.Sprintf("Lat:%.4f Lon:%.4f", lat, lon)
				}
			case *meshtastic.TelemetryData:
				if d.DeviceMetrics != nil {
					data = fmt.Sprintf("Batt:%d%% V:%.2f Ch:%.1f%% Up:%ds", 
						d.DeviceMetrics.BatteryLevel, d.DeviceMetrics.Voltage,
						d.DeviceMetrics.ChannelUtilization, d.DeviceMetrics.UptimeSeconds)
				} else if d.EnvironmentMetrics != nil {
					data = fmt.Sprintf("Temp:%.1f°C Hum:%.1f%% Press:%.1fhPa",
						d.EnvironmentMetrics.Temperature, d.EnvironmentMetrics.RelativeHumidity,
						d.EnvironmentMetrics.BarometricPressure)
				} else {
					data = "Telemetry"
				}
			case *meshtastic.RemoteHardwareMessage:
				data = fmt.Sprintf("%s: %s", d.Type.GetTypeName(), d.FormatGpioInfo())
			case *meshtastic.NodeInfo:
				if d.LongName != "" {
					// Sanitize usernames to prevent terminal display issues
					sanitizedLongName := utils.SanitizeForTerminal(d.LongName)
					sanitizedShortName := utils.SanitizeForTerminal(d.ShortName)
					data = fmt.Sprintf("%s (%s)", sanitizedLongName, sanitizedShortName)
				} else if d.ShortName != "" {
					data = utils.SanitizeForTerminal(d.ShortName)
				} else if d.ID != "" {
					data = d.ID
				} else {
					data = "Node Info"
				}
			}
		} else {
			// For unknown packets, show first few bytes of payload as hex
			if len(packet.Payload) > 0 {
				hexStr := ""
				maxBytes := len(packet.Payload)
				if maxBytes > 8 { // Limit to first 8 bytes for display
					maxBytes = 8
				}
				for i := 0; i < maxBytes; i++ {
					hexStr += fmt.Sprintf("%02x", packet.Payload[i])
				}
				if len(packet.Payload) > 8 {
					hexStr += "..."
				}
				data = fmt.Sprintf("Raw: %s (%d bytes)", hexStr, len(packet.Payload))
			} else {
				data = "Empty payload"
			}
		}
		
		if len(data) > 25 {
			data = data[:25] + "..."
		}

		// Format display based on packet type - use friendly names from NodeDB
		nodeDB := m.client.GetNodeDB()
		
		fromDisplay := packet.GetFromShortName(nodeDB)
		toDisplay := packet.GetToShortName(nodeDB)
		
		// Sanitize and truncate long names to fit in column width
		fromDisplay = utils.TruncateForDisplay(fromDisplay, 9)
		toDisplay = utils.TruncateForDisplay(toDisplay, 9)
		
		hopDisplay := packet.GetHopInfo()
		rssiDisplay := fmt.Sprintf("%.0f", float64(packet.RxRSSI))
		
		// Special formatting for device/CLI messages
		if packet.From == 0 && packet.To == 0xFFFFFFFF && packet.RxRSSI == 0 {
			fromDisplay = "DEVICE"
			toDisplay = "CLI"
			hopDisplay = "-"
			rssiDisplay = "-"
		}
		
		row := table.Row{
			packet.RxTime.Format("15:04:05"),
			fromDisplay,
			toDisplay,
			packet.GetTypeName(),
			fmt.Sprintf("%d", packet.Channel),
			hopDisplay,
			rssiDisplay,
			data,
		}
		rows = append(rows, row)
	}
	
	m.packetTable.SetRows(rows)
}

func (m *Model) onPacketReceived(packet *meshtastic.Packet) {
	// This will be called from a goroutine, so we need to send a message
	// to the main update loop via the packet channel
	select {
	case m.packetChan <- packet:
		// Successfully queued for UI processing
	default:
		// Channel is full, drop packet to avoid blocking
		m.logger.Println("Packet channel full, dropping packet for UI")
	}
}

// Messages

type tickMsg struct{}

// packetMsg wraps a Meshtastic packet for Bubble Tea's update loop
type packetMsg struct {
	Packet *meshtastic.Packet
}

// listenForPacketsCmd listens on a channel and emits packetMsg into Bubble Tea's loop
func listenForPacketsCmd(ch <-chan *meshtastic.Packet) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return packetMsg{Packet: p}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}
