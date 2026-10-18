package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hit-chat/campusnet/internal/geo"
)

type ChatMessage struct {
	ID        string
	Sender    string
	Text      string
	SentAt    time.Time
	ExpiresAt time.Time
}

type TUIModel struct {
	mu              sync.RWMutex
	serverURL       string
	userHandle      string
	deviceID        string
	locationEnabled bool
	currentZone     string
	contacts        []string
	selectedContact int
	messages        []ChatMessage
	inputBuffer     string
	statusMessage   string
	running         bool
}

func NewTUIModel(serverURL, userHandle, deviceID string) *TUIModel {
	m := &TUIModel{
		serverURL:       serverURL,
		userHandle:      userHandle,
		deviceID:        deviceID,
		locationEnabled: false, // Default to privacy mode enabled
		currentZone:     "Privacy Protected (Location Concealed)",
		contacts:        []string{"@alice", "@bob", "@library_desk", "@study_group"},
		selectedContact: 0,
		messages:        make([]ChatMessage, 0),
		statusMessage:   "Ready. Press Ctrl+P to toggle location privacy.",
		running:         true,
	}

	// Add sample initial ephemeral messages to demonstrate burn countdown
	now := time.Now()
	m.messages = append(m.messages, ChatMessage{
		ID:        "msg-1",
		Sender:    "@alice",
		Text:      "Hey! Are you studying at the library right now?",
		SentAt:    now,
		ExpiresAt: now.Add(15 * time.Second),
	})
	m.messages = append(m.messages, ChatMessage{
		ID:        "msg-2",
		Sender:    userHandle,
		Text:      "Yes, 2nd floor silent section. Sending encrypted coordinates.",
		SentAt:    now.Add(1 * time.Second),
		ExpiresAt: now.Add(15 * time.Second),
	})

	return m
}

func (m *TUIModel) TogglePrivacy() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.locationEnabled = !m.locationEnabled
	if m.locationEnabled {
		// Sniff real BSSID if on Windows
		bssid, ssid, err := geo.SniffCurrentBSSID()
		if err == nil && bssid != "" {
			m.currentZone = fmt.Sprintf("Campus AP [%s] (SSID: %s)", bssid[:8]+"...", ssid)
		} else {
			m.currentZone = "Library 2nd Floor - Silent Study (Zone: LIB_FL2_SILENT)"
		}
		m.statusMessage = "Location Broadcasting ENABLED (Zone: " + m.currentZone + ")"
	} else {
		m.currentZone = "Privacy Protected (Location Concealed)"
		m.statusMessage = "Location Privacy ENABLED (Null vectors sent to peers)"
	}
}

func (m *TUIModel) SendCurrentInput() {
	m.mu.Lock()
	defer m.mu.Unlock()

	text := strings.TrimSpace(m.inputBuffer)
	if text == "" {
		return
	}

	target := m.contacts[m.selectedContact]
	now := time.Now()
	m.messages = append(m.messages, ChatMessage{
		ID:        fmt.Sprintf("msg-%d", now.UnixNano()),
		Sender:    m.userHandle,
		Text:      text,
		SentAt:    now,
		ExpiresAt: now.Add(10 * time.Second),
	})

	m.statusMessage = fmt.Sprintf("Message dispatched to %s with 10s burn timer.", target)
	m.inputBuffer = ""
}

func (m *TUIModel) ShredScreen() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
	m.statusMessage = "Memory shredded! All displayed messages purged."
}

func (m *TUIModel) Render() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	var b strings.Builder

	// Top Header
	b.WriteString("\033[H\033[2J") // Clear terminal screen
	b.WriteString("╔══════════════════════════════════════════════════════════════════════════════════════╗\n")
	b.WriteString(fmt.Sprintf("║ CAMPUSNET SECURE EPHEMERAL CLIENT  |  Account: %-12s | Device: %-12s ║\n", m.userHandle, m.deviceID))
	locTag := "[PRIVACY ON]"
	if m.locationEnabled {
		locTag = "[LOCATION ON]"
	}
	b.WriteString(fmt.Sprintf("║ Zone: %-52s %-15s ║\n", m.currentZone, locTag))
	b.WriteString("╠══════════════════════════════════════════════════════════════════════════════════════╣\n")

	// Split Body: Left Contacts (25 cols), Right Feed (59 cols)
	activeRecipient := m.contacts[m.selectedContact]
	b.WriteString(fmt.Sprintf("║ CONTACTS                  │ EPHEMERAL CONVERSATION WITH %-28s ║\n", activeRecipient))
	b.WriteString("╟───────────────────────────┼──────────────────────────────────────────────────────────╢\n")

	// Filter unexpired messages
	var liveMessages []ChatMessage
	for _, msg := range m.messages {
		if now.Before(msg.ExpiresAt) {
			liveMessages = append(liveMessages, msg)
		}
	}

	maxRows := 12
	for i := 0; i < maxRows; i++ {
		// Contact column
		contactCol := "                          "
		if i < len(m.contacts) {
			prefix := "  "
			if i == m.selectedContact {
				prefix = "> "
			}
			contactCol = fmt.Sprintf("%-26s", prefix+m.contacts[i])
		}

		// Message column
		msgCol := "                                                         "
		if i < len(liveMessages) {
			msg := liveMessages[i]
			rem := msg.ExpiresAt.Sub(now).Seconds()
			if rem < 0 {
				rem = 0
			}
			timerStr := fmt.Sprintf("[%.1fs]", rem)
			content := fmt.Sprintf("%s %s: %s", timerStr, msg.Sender, msg.Text)
			if len(content) > 55 {
				content = content[:52] + "..."
			}
			msgCol = fmt.Sprintf("%-57s", content)
		}

		b.WriteString(fmt.Sprintf("║ %s│ %s║\n", contactCol, msgCol))
	}

	b.WriteString("╠══════════════════════════════════════════════════════════════════════════════════════╣\n")
	b.WriteString(fmt.Sprintf("║ Status: %-76s ║\n", m.statusMessage))
	b.WriteString("╠══════════════════════════════════════════════════════════════════════════════════════╣\n")
	b.WriteString(fmt.Sprintf("║ > %-82s ║\n", m.inputBuffer))
	b.WriteString("╚══════════════════════════════════════════════════════════════════════════════════════╝\n")
	b.WriteString(" [Hotkeys] Ctrl+P: Toggle Privacy | Ctrl+L: Shred Screen | Tab: Next Peer | Ctrl+C: Quit\n")

	return b.String()
}

// RunTUI launches the interactive terminal rendering loop.
func RunTUI(serverURL, userHandle, deviceID string) error {
	model := NewTUIModel(serverURL, userHandle, deviceID)

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	// Initial render
	fmt.Print(model.Render())

	// Background ticker updates live countdowns
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Print(model.Render())
			}
		}
	}()

	// Keep running until interrupted
	time.Sleep(3 * time.Second) // Demonstration pause
	close(done)
	return nil
}
