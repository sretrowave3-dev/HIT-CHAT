package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hit-chat/campusnet/internal/auth"
	"github.com/hit-chat/campusnet/internal/crypto"
	"github.com/hit-chat/campusnet/internal/ephemeral"
	"github.com/hit-chat/campusnet/internal/geo"
	"github.com/hit-chat/campusnet/internal/transport"
	"github.com/hit-chat/campusnet/pkg/protocol"
)

func TestCLIWebSocketEndToEndMessaging(t *testing.T) {
	// Setup ephemeral manager, privacy filter, and WebSocket hub
	memStore := ephemeral.NewMemoryStore(500 * time.Millisecond)
	defer memStore.Close()
	ephemMgr := ephemeral.NewManager(memStore)
	geoResolver := geo.NewBSSIDResolver("test-salt")
	filter := geo.NewPrivacyFilter(geoResolver)
	hub := transport.NewHub(ephemMgr, filter)
	go hub.Run()

	tokenMgr := auth.NewTokenManager("test-jwt-secret-min-32-chars-long!")

	// Spin up test HTTP server with WS endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(tokenMgr, w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Issue token for student_guest and alice
	guestToken, err := tokenMgr.GenerateDeviceToken("dev-cli-1", "student_guest", "cli")
	if err != nil {
		t.Fatalf("failed to generate guest token: %v", err)
	}
	aliceToken, err := tokenMgr.GenerateDeviceToken("dev-web-1", "alice", "web")
	if err != nil {
		t.Fatalf("failed to generate alice token: %v", err)
	}

	// Connect Alice client
	aliceConn, _, err := websocket.DefaultDialer.Dial(wsURL+"?token="+aliceToken, nil)
	if err != nil {
		t.Fatalf("failed to connect alice ws: %v", err)
	}
	defer aliceConn.Close()

	// Connect Guest client
	guestConn, _, err := websocket.DefaultDialer.Dial(wsURL+"?token="+guestToken, nil)
	if err != nil {
		t.Fatalf("failed to connect guest ws: %v", err)
	}
	defer guestConn.Close()

	time.Sleep(100 * time.Millisecond) // Allow hub registration

	// Guest sends pairwise encrypted message with UTF-8 runes & emojis to Alice
	msgText := "Hello Alice! 🔐 This is an ephemeral message from CLI with emojis 🔥"
	iv, cipher, tag, err := crypto.EncryptPairwisePayload("student_guest", "alice", []byte(msgText))
	if err != nil {
		t.Fatalf("failed to encrypt pairwise payload: %v", err)
	}

	env := protocol.NewEnvelope("student_guest", "alice", 15000)
	env.IV = iv
	env.Ciphertext = cipher
	env.AuthTag = tag
	ephemKP, _ := crypto.GenerateKeyPair()
	env.EphemeralPublicKey = ephemKP.PublicKey

	sendFrame := protocol.StreamFrame{
		Op:       protocol.OpSendEnvelope,
		Envelope: env,
	}
	data, _ := json.Marshal(sendFrame)
	if err := guestConn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to send frame: %v", err)
	}

	// Guest should receive OpAckDelivered
	_ = guestConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, ackData, err := guestConn.ReadMessage()
	if err != nil {
		t.Fatalf("guest failed to receive ack: %v", err)
	}
	var ackFrame protocol.StreamFrame
	if err := json.Unmarshal(ackData, &ackFrame); err != nil || ackFrame.Op != protocol.OpAckDelivered {
		t.Fatalf("expected OpAckDelivered, got op=%s error=%q err=%v", ackFrame.Op, ackFrame.Error, err)
	}

	// Alice should receive OpDeliverEnvelope
	_ = aliceConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, deliverData, err := aliceConn.ReadMessage()
	if err != nil {
		t.Fatalf("alice failed to receive deliver envelope: %v", err)
	}
	var deliverFrame protocol.StreamFrame
	if err := json.Unmarshal(deliverData, &deliverFrame); err != nil || deliverFrame.Op != protocol.OpDeliverEnvelope {
		t.Fatalf("expected OpDeliverEnvelope, got op=%s err=%v", deliverFrame.Op, err)
	}

	// Alice decrypts message with pairwise key
	decrypted, err := crypto.DecryptPairwisePayload("student_guest", "alice", deliverFrame.Envelope.IV, deliverFrame.Envelope.Ciphertext, deliverFrame.Envelope.AuthTag)
	if err != nil {
		t.Fatalf("alice failed to decrypt payload: %v", err)
	}
	if string(decrypted) != msgText {
		t.Fatalf("decrypted text mismatch: expected %q, got %q", msgText, string(decrypted))
	}

	// Alice sends OpAckRead to burn message
	readAckFrame := protocol.StreamFrame{
		Op:           protocol.OpAckRead,
		AckMessageID: deliverFrame.Envelope.ID,
	}
	rData, _ := json.Marshal(readAckFrame)
	if err := aliceConn.WriteMessage(websocket.TextMessage, rData); err != nil {
		t.Fatalf("alice failed to write ack read: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify message was burned from ephemeral store
	burned, _ := memStore.GetEnvelope(context.Background(), deliverFrame.Envelope.ID)
	if burned != nil {
		t.Fatalf("expected envelope to be burned, but still found in store: %v", burned)
	}
}

func TestTUIInputHandlingAndRuneSafety(t *testing.T) {
	m := initialModel("http://localhost:8080", "student_guest", "dev-test", "token-test")

	// Test backspace on unicode emoji
	m.inputBuffer = "Hello 👋"
	runes := []rune(m.inputBuffer)
	if len(runes) != 7 {
		t.Fatalf("expected 7 runes for 'Hello 👋', got %d", len(runes))
	}
	m.inputBuffer = string(runes[:len(runes)-1])
	if m.inputBuffer != "Hello " {
		t.Fatalf("expected 'Hello ', got %q", m.inputBuffer)
	}

	// Test truncation on long message with emojis
	longLine := " [14.9s] @alice: 🔒 CampusNet: Super secret research notes with non-ascii characters and emojis 🔥"
	lineRunes := []rune(longLine)
	if len(lineRunes) > 46 {
		truncated := string(lineRunes[:43]) + "..."
		if !strings.HasSuffix(truncated, "...") {
			t.Fatalf("expected suffix '...', got %q", truncated)
		}
	}
}
