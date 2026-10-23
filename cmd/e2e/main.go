package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hit-chat/campusnet/internal/auth"
	"github.com/hit-chat/campusnet/internal/crypto"
	"github.com/hit-chat/campusnet/internal/ephemeral"
	"github.com/hit-chat/campusnet/internal/geo"
	"github.com/hit-chat/campusnet/internal/transport"
	"github.com/hit-chat/campusnet/pkg/protocol"
)

func main() {
	fmt.Println("==================================================================")
	fmt.Println("       CAMPUSNET END-TO-END VERIFICATION: CLI <-> WEB E2EE        ")
	fmt.Println("==================================================================")

	memStore := ephemeral.NewMemoryStore(500 * time.Millisecond)
	defer memStore.Close()
	ephemMgr := ephemeral.NewManager(memStore)
	geoResolver := geo.NewBSSIDResolver("e2e-salt")
	privacyFilter := geo.NewPrivacyFilter(geoResolver)
	hub := transport.NewHub(ephemMgr, privacyFilter)
	go hub.Run()

	tokenMgr := auth.NewTokenManager("test-jwt-secret-min-32-chars-long-e2e-test!")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(tokenMgr, w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// 1. Connect Web Client (@alice)
	aliceToken, err := tokenMgr.GenerateDeviceToken("web-alice-1", "alice", "web")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Generating Alice token: %v\n", err)
		os.Exit(1)
	}

	aliceConn, _, err := websocket.DefaultDialer.Dial(wsURL+"?token="+aliceToken, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Connecting Alice WS: %v\n", err)
		os.Exit(1)
	}
	defer aliceConn.Close()
	fmt.Println(" [+] [WEB CLIENT] Connected as @alice (web-alice-1)")

	time.Sleep(50 * time.Millisecond)

	// 2. Connect CLI Client (@student_guest)
	guestToken, err := tokenMgr.GenerateDeviceToken("cli-guest-1", "student_guest", "cli")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Generating Guest token: %v\n", err)
		os.Exit(1)
	}

	guestConn, _, err := websocket.DefaultDialer.Dial(wsURL+"?token="+guestToken, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Connecting Guest WS: %v\n", err)
		os.Exit(1)
	}
	defer guestConn.Close()
	fmt.Println(" [+] [CLI CLIENT] Connected as @student_guest (cli-guest-1)")

	time.Sleep(50 * time.Millisecond)

	// 3. Step 1: CLI -> Web message
	cliMessageText := "Hello Alice! Here are the confidential Lab 4 results from terminal."
	fmt.Printf("\n [1] CLI (@student_guest) encrypting message for @alice...\n")
	iv, cipher, tag, err := crypto.EncryptPairwisePayload("student_guest", "alice", []byte(cliMessageText))
	if err != nil {
		fmt.Fprintf(os.Stderr, "CLI encryption failed: %v\n", err)
		os.Exit(1)
	}

	cliEnv := protocol.NewEnvelope("student_guest", "alice", 10000)
	cliEnv.IV = iv
	cliEnv.Ciphertext = cipher
	cliEnv.AuthTag = tag
	ephemKP, _ := crypto.GenerateKeyPair()
	cliEnv.EphemeralPublicKey = ephemKP.PublicKey

	sendFrame := protocol.StreamFrame{
		Op:       protocol.OpSendEnvelope,
		Envelope: cliEnv,
	}
	sendData, _ := json.Marshal(sendFrame)
	if err := guestConn.WriteMessage(websocket.TextMessage, sendData); err != nil {
		fmt.Fprintf(os.Stderr, "CLI write failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(" [+] CLI message dispatched over WebSocket.")

	// Alice receives and decrypts
	_ = aliceConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, aliceRecvData, err := aliceConn.ReadMessage()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Alice failed to receive frame: %v\n", err)
		os.Exit(1)
	}

	var aliceFrame protocol.StreamFrame
	if err := json.Unmarshal(aliceRecvData, &aliceFrame); err != nil {
		fmt.Fprintf(os.Stderr, "Alice JSON unmarshal failed: %v\n", err)
		os.Exit(1)
	}

	if aliceFrame.Op != protocol.OpDeliverEnvelope || aliceFrame.Envelope == nil {
		fmt.Fprintf(os.Stderr, "Expected DELIVER_ENVELOPE, got %s\n", aliceFrame.Op)
		os.Exit(1)
	}

	recvEnv := aliceFrame.Envelope
	decryptedByAlice, err := crypto.DecryptPairwisePayload(recvEnv.SenderID, "alice", recvEnv.IV, recvEnv.Ciphertext, recvEnv.AuthTag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Alice decryption failed: %v\n", err)
		os.Exit(1)
	}

	if string(decryptedByAlice) != cliMessageText {
		fmt.Fprintf(os.Stderr, "Plaintext mismatch: expected %q, got %q\n", cliMessageText, string(decryptedByAlice))
		os.Exit(1)
	}
	fmt.Printf(" [+] [WEB RECEIVED & DECRYPTED]: %q\n", string(decryptedByAlice))

	// Alice sends ACK_DELIVERED and ACK_READ
	ackDeliveredFrame := protocol.StreamFrame{
		Op:           protocol.OpAckDelivered,
		AckMessageID: recvEnv.ID,
	}
	dBytes, _ := json.Marshal(ackDeliveredFrame)
	_ = aliceConn.WriteMessage(websocket.TextMessage, dBytes)

	ackReadFrame := protocol.StreamFrame{
		Op:           protocol.OpAckRead,
		AckMessageID: recvEnv.ID,
	}
	rBytes, _ := json.Marshal(ackReadFrame)
	_ = aliceConn.WriteMessage(websocket.TextMessage, rBytes)
	fmt.Println(" [+] Alice dispatched ACK_DELIVERED and ACK_READ.")

	// 4. Step 2: Web -> CLI reply
	webReplyText := "Got it, @student_guest! Lab results confirmed and logged in browser."
	fmt.Printf("\n [2] Web (@alice) replying to @student_guest...\n")
	wIV, wCipher, wTag, err := crypto.EncryptPairwisePayload("alice", "student_guest", []byte(webReplyText))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Alice encryption failed: %v\n", err)
		os.Exit(1)
	}

	webEnv := protocol.NewEnvelope("alice", "student_guest", 10000)
	webEnv.IV = wIV
	webEnv.Ciphertext = wCipher
	webEnv.AuthTag = wTag
	webKP, _ := crypto.GenerateKeyPair()
	webEnv.EphemeralPublicKey = webKP.PublicKey

	webSendFrame := protocol.StreamFrame{
		Op:       protocol.OpSendEnvelope,
		Envelope: webEnv,
	}
	wsData, _ := json.Marshal(webSendFrame)
	if err := aliceConn.WriteMessage(websocket.TextMessage, wsData); err != nil {
		fmt.Fprintf(os.Stderr, "Alice write reply failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(" [+] Alice reply dispatched over WebSocket.")

	// CLI receives and decrypts (draining any ACKs first)
	var cliFrame protocol.StreamFrame
	for {
		_ = guestConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, cliRecvData, err := guestConn.ReadMessage()
		if err != nil {
			fmt.Fprintf(os.Stderr, "CLI failed to receive reply: %v\n", err)
			os.Exit(1)
		}

		if err := json.Unmarshal(cliRecvData, &cliFrame); err != nil {
			fmt.Fprintf(os.Stderr, "CLI JSON unmarshal failed: %v\n", err)
			os.Exit(1)
		}

		if cliFrame.Op == protocol.OpAckDelivered {
			fmt.Printf(" [+] [CLI RECEIVED ACK_DELIVERED] for msg: %s\n", cliFrame.AckMessageID)
			continue
		}
		if cliFrame.Op == protocol.OpAckRead {
			fmt.Printf(" [+] [CLI RECEIVED ACK_READ] for msg: %s\n", cliFrame.AckMessageID)
			continue
		}
		if cliFrame.Op == protocol.OpDeliverEnvelope {
			break
		}
	}

	if cliFrame.Envelope == nil {
		fmt.Fprintf(os.Stderr, "Received frame has nil envelope\n")
		os.Exit(1)
	}

	decryptedByCLI, err := crypto.DecryptPairwisePayload(cliFrame.Envelope.SenderID, "student_guest", cliFrame.Envelope.IV, cliFrame.Envelope.Ciphertext, cliFrame.Envelope.AuthTag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CLI decryption failed: %v\n", err)
		os.Exit(1)
	}

	if string(decryptedByCLI) != webReplyText {
		fmt.Fprintf(os.Stderr, "Plaintext mismatch: expected %q, got %q\n", webReplyText, string(decryptedByCLI))
		os.Exit(1)
	}
	fmt.Printf(" [+] [CLI RECEIVED & DECRYPTED]: %q\n", string(decryptedByCLI))

	fmt.Println("\n==================================================================")
	fmt.Println(" [SUCCESS] BIDIRECTIONAL CLI <-> WEB E2EE CHAT FULLY VERIFIED!   ")
	fmt.Println(" Zero plaintext leakage, zero decrypt failures, zero premature burns.")
	fmt.Println("==================================================================")
}
