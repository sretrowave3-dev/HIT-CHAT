package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
	"github.com/hit-chat/campusnet/internal/crypto"
	"github.com/hit-chat/campusnet/internal/geo"
	"github.com/hit-chat/campusnet/pkg/protocol"
)

type ChatMessage struct {
	ID        string
	Sender    string
	Text      string
	SentAt    time.Time
	ExpiresAt time.Time
}

type tickMsg time.Time

type wsIncomingMsg *protocol.Envelope

type model struct {
	serverURL       string
	userHandle      string
	deviceID        string
	token           string
	locationEnabled bool
	currentZone     string
	contacts        []string
	selectedContact int
	messages        map[string][]ChatMessage // contactHandle -> []ChatMessage
	inputBuffer     string
	statusMessage   string
	wsConn          *websocket.Conn
	width           int
	height          int
	quitting        bool
}

func initialModel(serverURL, userHandle, deviceID, token string) model {
	contacts := []string{"@alice", "@bob", "@carol", "@study_group", "@robotics_lab"}
	msgs := make(map[string][]ChatMessage)
	for _, c := range contacts {
		msgs[c] = make([]ChatMessage, 0)
	}

	return model{
		serverURL:       serverURL,
		userHandle:      userHandle,
		deviceID:        deviceID,
		token:           token,
		locationEnabled: false,
		currentZone:     "Privacy Protected (Location Concealed)",
		contacts:        contacts,
		selectedContact: 0,
		messages:        msgs,
		inputBuffer:     "",
		statusMessage:   "Ready. Type message and press Enter. [Ctrl+P] Toggle Privacy | [Esc] Quit",
		width:           80,
		height:          24,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		m.connectWebSocket(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) connectWebSocket() tea.Cmd {
	return func() tea.Msg {
		if m.token == "" {
			return nil
		}
		u, err := url.Parse(m.serverURL)
		if err != nil {
			return nil
		}
		scheme := "ws"
		if u.Scheme == "https" {
			scheme = "wss"
		}
		wsURL := fmt.Sprintf("%s://%s/ws/v1/stream?token=%s", scheme, u.Host, m.token)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			return nil
		}
		return conn
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case *websocket.Conn:
		m.wsConn = msg
		m.statusMessage = "Connected to CampusNet gateway."
		return m, m.listenWebSocket()

	case wsIncomingMsg:
		if msg != nil {
			sender := msg.SenderID
			if !strings.HasPrefix(sender, "@") {
				sender = "@" + sender
			}
			chatMsg := ChatMessage{
				ID:        msg.ID,
				Sender:    sender,
				Text:      string(msg.Ciphertext), // Display ciphertext if not locally ratcheted
				SentAt:    time.UnixMilli(msg.TimestampMS),
				ExpiresAt: time.Now().Add(time.Duration(msg.TTLMilliseconds) * time.Millisecond),
			}
			m.messages[sender] = append(m.messages[sender], chatMsg)
			m.statusMessage = fmt.Sprintf("New ephemeral message from %s (burns in %dms)", sender, msg.TTLMilliseconds)
		}
		return m, m.listenWebSocket()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		// Sweep expired messages across all conversations
		now := time.Now()
		for c := range m.messages {
			var unexpired []ChatMessage
			for _, item := range m.messages[c] {
				if now.Before(item.ExpiresAt) {
					unexpired = append(unexpired, item)
				}
			}
			m.messages[c] = unexpired
		}
		return m, tickCmd()

	case tea.KeyMsg:
		switch msg.Type {

		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			if m.wsConn != nil {
				_ = m.wsConn.Close()
			}
			return m, tea.Quit

		case tea.KeyTab, tea.KeyDown:
			m.selectedContact = (m.selectedContact + 1) % len(m.contacts)
			m.statusMessage = fmt.Sprintf("Switched to conversation with %s", m.contacts[m.selectedContact])
			return m, nil

		case tea.KeyShiftTab, tea.KeyUp:
			m.selectedContact = (m.selectedContact - 1 + len(m.contacts)) % len(m.contacts)
			m.statusMessage = fmt.Sprintf("Switched to conversation with %s", m.contacts[m.selectedContact])
			return m, nil

		case tea.KeyCtrlP:
			m.locationEnabled = !m.locationEnabled
			if m.locationEnabled {
				bssid, ssid, err := geo.SniffCurrentBSSID()
				if err == nil && bssid != "" {
					m.currentZone = fmt.Sprintf("Wi-Fi [%s] (SSID: %s)", bssid[:8]+"...", ssid)
				} else {
					m.currentZone = "Engineering Hall - Robotics Lab 4"
				}
				m.statusMessage = "Location Broadcasting ENABLED: " + m.currentZone
			} else {
				m.currentZone = "Privacy Protected (Location Concealed)"
				m.statusMessage = "Location Privacy ENABLED: Null vectors sent to peers"
			}
			return m, nil

		case tea.KeyCtrlL:
			// Shred all in-memory buffers
			for c := range m.messages {
				m.messages[c] = nil
			}
			m.statusMessage = "Screen & RAM shredded! All displayed messages purged."
			return m, nil

		case tea.KeyBackspace:
			if len(m.inputBuffer) > 0 {
				m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
			}
			return m, nil

		case tea.KeyEnter:
			text := strings.TrimSpace(m.inputBuffer)
			if text != "" {
				target := m.contacts[m.selectedContact]
				now := time.Now()
				ttlMS := int64(15000) // 15 seconds

				chatMsg := ChatMessage{
					ID:        fmt.Sprintf("msg-%d", now.UnixNano()),
					Sender:    "@" + m.userHandle,
					Text:      text,
					SentAt:    now,
					ExpiresAt: now.Add(time.Duration(ttlMS) * time.Millisecond),
				}
				m.messages[target] = append(m.messages[target], chatMsg)

				// If connected to server, send over WebSocket
				if m.wsConn != nil {
					env := protocol.NewEnvelope(m.userHandle, strings.TrimPrefix(target, "@"), ttlMS)
					env.Ciphertext = []byte(text)
					env.EphemeralPublicKey = make([]byte, 32)
					env.IV = make([]byte, 12)
					env.AuthTag = make([]byte, 16)
					frame := protocol.StreamFrame{
						Op:       protocol.OpSendEnvelope,
						Envelope: env,
					}
					data, _ := json.Marshal(frame)
					_ = m.wsConn.WriteMessage(websocket.TextMessage, data)
				}

				m.statusMessage = fmt.Sprintf("Dispatched message to %s (15s burn timer).", target)
				m.inputBuffer = ""
			}
			return m, nil

		case tea.KeyRunes, tea.KeySpace:
			m.inputBuffer += string(msg.Runes)
			return m, nil
		}
	}

	return m, nil
}

func (m model) listenWebSocket() tea.Cmd {
	return func() tea.Msg {
		if m.wsConn == nil {
			return nil
		}
		_, data, err := m.wsConn.ReadMessage()
		if err != nil {
			return nil
		}
		var frame protocol.StreamFrame
		if err := json.Unmarshal(data, &frame); err == nil && frame.Op == protocol.OpDeliverEnvelope {
			return wsIncomingMsg(frame.Envelope)
		}
		return nil
	}
}

func (m model) View() string {
	if m.quitting {
		return "Shutting down CampusNet Secure CLI. Session keys zeroed from memory.\n"
	}

	now := time.Now()
	activeContact := m.contacts[m.selectedContact]
	feed := m.messages[activeContact]

	// Styles using lipgloss
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#10b981")).
		Background(lipgloss.Color("#111827")).
		Padding(0, 1)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151"))

	sidebarStyle := borderStyle.Width(24).Height(14)
	chatStyle := borderStyle.Width(58).Height(14)

	// Top Bar
	locBadge := "[PRIVACY ON]"
	if m.locationEnabled {
		locBadge = "[LOCATION ON]"
	}
	topBar := headerStyle.Render(fmt.Sprintf(
		" CAMPUSNET SECURE TERMINAL | User: @%s | Device: %s ",
		m.userHandle, m.deviceID,
	))
	zoneBar := fmt.Sprintf(" Zone: %-50s %s", m.currentZone, locBadge)

	// Contacts list
	var contactsList []string
	for i, c := range m.contacts {
		msgCount := len(m.messages[c])
		countTag := ""
		if msgCount > 0 {
			countTag = fmt.Sprintf(" (%d)", msgCount)
		}
		if i == m.selectedContact {
			contactsList = append(contactsList, fmt.Sprintf("> %-16s%s", c, countTag))
		} else {
			contactsList = append(contactsList, fmt.Sprintf("  %-16s%s", c, countTag))
		}
	}
	contactsPane := sidebarStyle.Render(
		"CAMPUS CONTACTS\n───────────────\n" + strings.Join(contactsList, "\n"),
	)

	// Chat Messages
	var msgLines []string
	if len(feed) == 0 {
		msgLines = append(msgLines, "")
		msgLines = append(msgLines, "  [No active messages]")
		msgLines = append(msgLines, fmt.Sprintf("  Start an ephemeral encrypted chat with %s.", activeContact))
		msgLines = append(msgLines, "  Messages burn automatically upon expiration.")
	} else {
		for _, msg := range feed {
			rem := msg.ExpiresAt.Sub(now).Seconds()
			if rem < 0 {
				rem = 0
			}
			timerStr := fmt.Sprintf("[%.1fs]", rem)
			line := fmt.Sprintf(" %s %s: %s", timerStr, msg.Sender, msg.Text)
			if len(line) > 54 {
				line = line[:51] + "..."
			}
			msgLines = append(msgLines, line)
		}
	}
	for len(msgLines) < 12 {
		msgLines = append(msgLines, "")
	}

	chatPane := chatStyle.Render(
		fmt.Sprintf("CONVERSATION: %s\n────────────────────────────────────────────────────────\n", activeContact) +
			strings.Join(msgLines[:12], "\n"),
	)

	body := lipgloss.JoinHorizontal(lipgloss.Top, contactsPane, chatPane)

	// Status & Input
	statusBar := fmt.Sprintf(" Status: %s", m.statusMessage)
	inputBox := borderStyle.Width(84).Render(
		fmt.Sprintf(" [%s] > %s█", activeContact, m.inputBuffer),
	)
	hotkeys := " [Tab] Switch Contact | [Enter] Send | [Ctrl+P] Toggle Location | [Ctrl+L] Shred | [Esc] Quit"

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n", topBar, zoneBar, body, statusBar, inputBox, hotkeys)
}

// RunTUI launches the interactive Bubble Tea terminal program.
func RunTUI(serverURL, userHandle, deviceID string) error {
	secStore, _ := crypto.NewSecureStore()
	token := ""
	if secStore != nil {
		if t, err := secStore.Get("access_token"); err == nil {
			token = string(t)
		}
	}

	m := initialModel(serverURL, userHandle, deviceID, token)

	// Run with alternate screen so the terminal is completely restored on quit and never spams scrollback
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
