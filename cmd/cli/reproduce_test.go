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

func TestReproduceCLIToWebBug(t *testing.T) {
	memStore := ephemeral.NewMemoryStore(500 * time.Millisecond)
	defer memStore.Close()
	ephemMgr := ephemeral.NewManager(memStore)
	geoResolver := geo.NewBSSIDResolver("test-salt")
	filter := geo.NewPrivacyFilter(geoResolver)
	hub := transport.NewHub(ephemMgr, filter)
	go hub.Run()

	tokenMgr := auth.NewTokenManager("test-jwt-secret-min-32-chars-long!")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(tokenMgr, w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// 1. Web client (Alice) connects
	aliceToken, err := tokenMgr.GenerateDeviceToken("web-dev-1", "alice", "web")
	if err != nil {
		t.Fatal(err)
	}

	aliceConn, _, err := websocket.DefaultDialer.Dial(wsURL+"?token="+aliceToken, nil)
	if err != nil {
		t.Fatalf("failed to connect alice ws: %v", err)
	}
	defer aliceConn.Close()

	time.Sleep(50 * time.Millisecond)

	// 2. CLI client (student_guest) sends message to @alice
	guestToken, err := tokenMgr.GenerateDeviceToken("cli-dev-1", "student_guest", "cli")
	if err != nil {
		t.Fatal(err)
	}

	guestConn, _, err := websocket.DefaultDialer.Dial(wsURL+"?token="+guestToken, nil)
	if err != nil {
		t.Fatalf("failed to connect guest ws: %v", err)
	}
	defer guestConn.Close()

	time.Sleep(50 * time.Millisecond)

	msgText := "Hello Alice! Here are the notes."
	iv, cipher, tag, err := crypto.EncryptPairwisePayload("student_guest", "alice", []byte(msgText))
	if err != nil {
		t.Fatal(err)
	}

	env := protocol.NewEnvelope("student_guest", "alice", 10000)
	env.IV = iv
	env.Ciphertext = cipher
	env.AuthTag = tag
	ephemKP, _ := crypto.GenerateKeyPair()
	env.EphemeralPublicKey = ephemKP.PublicKey

	frame := protocol.StreamFrame{
		Op:       protocol.OpSendEnvelope,
		Envelope: env,
	}
	data, _ := json.Marshal(frame)
	if err := guestConn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatal(err)
	}

	// 3. Alice receives frame
	_ = aliceConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := aliceConn.ReadMessage()
	if err != nil {
		t.Fatalf("alice failed to read message: %v", err)
	}

	t.Logf("RAW JSON DELIVERED TO ALICE: %s", string(msg))

	var deliverFrame protocol.StreamFrame
	if err := json.Unmarshal(msg, &deliverFrame); err != nil {
		t.Fatal(err)
	}

	delEnv := deliverFrame.Envelope
	t.Logf("delEnv: SenderID=%q RecipientID=%q IV=%x Cipher=%x Tag=%x",
		delEnv.SenderID, delEnv.RecipientID, delEnv.IV, delEnv.Ciphertext, delEnv.AuthTag)

	// Verify server stored it in ephemeral store
	saved, err := memStore.GetEnvelope(context.Background(), delEnv.ID)
	if err != nil || saved == nil {
		t.Fatalf("saved envelope not found in store: %v", err)
	}
	t.Logf("Store has envelope: %s (expires in %v)", saved.ID, saved.RemainingTTL())
}
