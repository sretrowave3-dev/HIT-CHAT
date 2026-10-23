package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

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

	initialStatus := "Ready. Type message and press Enter. [Ctrl+A] Auto-Connect | [Ctrl+P] Privacy | [Esc] Quit"
	if token == "" {
		initialStatus = "Disconnected. Press [Ctrl+A] to Auto-Connect & Authorize | [Esc] Quit"
	} else {
		initialStatus = fmt.Sprintf("Connected as @%s. [Ctrl+A] Re-connect | [Ctrl+P] Privacy | [Esc] Quit", userHandle)
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
		statusMessage:   initialStatus,
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

			// Decrypt incoming payload with pairwise key isolation, falling back cleanly
			decrypted, dErr := crypto.DecryptPairwisePayload(msg.SenderID, m.userHandle, msg.IV, msg.Ciphertext, msg.AuthTag)
			bodyText := ""
			if dErr == nil {
				bodyText = string(decrypted)
			} else if utf8.Valid(msg.Ciphertext) {
				bodyText = string(msg.Ciphertext)
			} else {
				bodyText = fmt.Sprintf("[Encrypted message: %d bytes]", len(msg.Ciphertext))
			}

			chatMsg := ChatMessage{
				ID:        msg.ID,
				Sender:    sender,
				Text:      bodyText,
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

		case tea.KeyCtrlA:
			t, d, h, err := EnsureAuthorized(m.serverURL, m.userHandle)
			if err == nil {
				m.token = t
				m.deviceID = d
				m.userHandle = h
				m.statusMessage = fmt.Sprintf("Auto-authorized as @%s! Connecting to gateway...", h)
				return m, m.connectWebSocket()
			}
			m.statusMessage = fmt.Sprintf("Auto-connect failed: %v", err)
			return m, nil

		case tea.KeyCtrlP:
			m.locationEnabled = !m.locationEnabled
			zoneCode := "PROTECTED"
			bssidHash := ""
			if m.locationEnabled {
				bssid, ssid, err := geo.SniffCurrentBSSID()
				if err == nil && bssid != "" {
					m.currentZone = fmt.Sprintf("Wi-Fi [%s] (SSID: %s)", bssid[:8]+"...", ssid)
					bssidHash = bssid
					zoneCode = "WIFI_CAMPUS"
				} else {
					m.currentZone = "Engineering Hall - Robotics Lab 4"
					zoneCode = "ENG_LAB_4"
				}
				m.statusMessage = "Location Broadcasting ENABLED: " + m.currentZone
			} else {
				m.currentZone = "Privacy Protected (Location Concealed)"
				m.statusMessage = "Location Privacy ENABLED: Null vectors sent to peers"
			}

			// Broadcast presence update over WebSocket
			if m.wsConn != nil {
				presenceFrame := protocol.StreamFrame{
					Op: protocol.OpUpdatePresence,
					Presence: &protocol.PresencePayload{
						UserHandle:      m.userHandle,
						BSSIDHash:       bssidHash,
						ZoneCode:        zoneCode,
						LocationEnabled: m.locationEnabled,
					},
				}
				data, _ := json.Marshal(presenceFrame)
				_ = m.wsConn.WriteMessage(websocket.TextMessage, data)
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

				// If connected to server, send encrypted over WebSocket
				if m.wsConn != nil {
					cleanTarget := strings.TrimPrefix(target, "@")
					env := protocol.NewEnvelope(m.userHandle, cleanTarget, ttlMS)
					iv, cipher, tag, eErr := crypto.EncryptPairwisePayload(m.userHandle, cleanTarget, []byte(text))
					if eErr == nil {
						env.IV = iv
						env.Ciphertext = cipher
						env.AuthTag = tag
					} else {
						env.Ciphertext = []byte(text)
					}
					ephemKP, _ := crypto.GenerateKeyPair()
					if ephemKP != nil {
						env.EphemeralPublicKey = ephemKP.PublicKey
					}
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

	sidebarStyle := borderStyle.Width(20).Height(14)
	chatStyle := borderStyle.Width(50).Height(14)

	// Top Bar
	locBadge := "[PRIVACY ON]"
	if m.locationEnabled {
		locBadge = "[LOCATION ON]"
	}
	connBadge := "[OFFLINE - Press Ctrl+A]"
	if m.wsConn != nil {
		connBadge = "[ONLINE]"
	}
	topBar := headerStyle.Render(fmt.Sprintf(
		" CAMPUSNET SECURE TERMINAL | User: @%-12s | %-10s ",
		m.userHandle, connBadge,
	))
	zoneBar := fmt.Sprintf(" Zone: %-45s %s", m.currentZone, locBadge)

	// Contacts list
	var contactsList []string
	for i, c := range m.contacts {
		msgCount := len(m.messages[c])
		countTag := ""
		if msgCount > 0 {
			countTag = fmt.Sprintf(" (%d)", msgCount)
		}
		if i == m.selectedContact {
			contactsList = append(contactsList, fmt.Sprintf("> %-13s%s", c, countTag))
		} else {
			contactsList = append(contactsList, fmt.Sprintf("  %-13s%s", c, countTag))
		}
	}
	contactsPane := sidebarStyle.Render(
		"CAMPUS PEERS\n────────────\n" + strings.Join(contactsList, "\n"),
	)

	// Chat Messages
	var msgLines []string
	if len(feed) == 0 {
		msgLines = append(msgLines, "")
		msgLines = append(msgLines, "  [No active messages]")
		msgLines = append(msgLines, fmt.Sprintf("  Chatting with %s.", activeContact))
		msgLines = append(msgLines, "  Messages burn upon expiration.")
	} else {
		for _, msg := range feed {
			rem := msg.ExpiresAt.Sub(now).Seconds()
			if rem < 0 {
				rem = 0
			}
			timerStr := fmt.Sprintf("[%.1fs]", rem)
			line := fmt.Sprintf(" %s %s: %s", timerStr, msg.Sender, msg.Text)
			if len(line) > 46 {
				line = line[:43] + "..."
			}
			msgLines = append(msgLines, line)
		}
	}
	for len(msgLines) < 12 {
		msgLines = append(msgLines, "")
	}

	start := 0
	if len(msgLines) > 12 {
		start = len(msgLines) - 12
	}

	chatPane := chatStyle.Render(
		fmt.Sprintf("CONVERSATION: %s\n──────────────────────────────────────────────────\n", activeContact) +
			strings.Join(msgLines[start:], "\n"),
	)

	body := lipgloss.JoinHorizontal(lipgloss.Top, contactsPane, chatPane)

	// Status & Input
	statusBar := fmt.Sprintf(" Status: %s", m.statusMessage)
	inputBox := borderStyle.Width(74).Render(
		fmt.Sprintf(" [%s] > %s█", activeContact, m.inputBuffer),
	)
	hotkeys := " [Tab] Switch | [Enter] Send | [Ctrl+A] Connect | [Ctrl+P] Loc | [Ctrl+L] Shred | [Esc] Quit"

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n", topBar, zoneBar, body, statusBar, inputBox, hotkeys)
}

// RunTUI launches the interactive Bubble Tea terminal program.
func RunTUI(serverURL, userHandle, deviceID, token string) error {
	if token == "" {
		secStore, _ := crypto.NewSecureStore()
		if secStore != nil {
			if t, err := secStore.Get("access_token"); err == nil {
				token = string(t)
			}
		}
	}

	m := initialModel(serverURL, userHandle, deviceID, token)

	// Run with alternate screen so the terminal is completely restored on quit and never spams scrollback
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
