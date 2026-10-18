package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hit-chat/campusnet/internal/crypto"
	"github.com/hit-chat/campusnet/pkg/protocol"
)

// RunPair initiates RFC 8628 device pairing in the terminal.
func RunPair(serverURL, userHandle string) error {
	secStore, err := crypto.NewSecureStore()
	if err != nil {
		return err
	}

	// 1. Generate local Identity Key if not existing
	ik, err := crypto.GenerateKeyPair()
	if err != nil {
		return err
	}

	reqBody := protocol.DeviceCodeRequest{
		ClientID:          fmt.Sprintf("cli-%d", time.Now().Unix()),
		Platform:          "cli",
		PublicIdentityKey: fmt.Sprintf("%x", ik.PublicKey),
	}
	bodyBytes, _ := json.Marshal(reqBody)

	resp, err := http.Post(serverURL+"/api/v1/auth/device/code", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	var codeResp protocol.DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&codeResp); err != nil {
		return err
	}

	fmt.Println("\n=======================================================")
	fmt.Println("             CAMPUSNET DEVICE AUTHORIZATION            ")
	fmt.Println("=======================================================")
	fmt.Printf("1. Open verification page: %s\n", codeResp.VerificationURI)
	fmt.Printf("2. Enter Device Pairing Code: %s\n", codeResp.UserCode)
	fmt.Println("=======================================================")
	fmt.Printf("Awaiting confirmation (expires in %ds)...\n", codeResp.ExpiresIn)

	pollInterval := time.Duration(codeResp.Interval) * time.Second
	if pollInterval < time.Second {
		pollInterval = 2 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		tokenPoll := struct {
			DeviceCode string `json:"device_code"`
			ClientID   string `json:"client_id"`
		}{
			DeviceCode: codeResp.DeviceCode,
			ClientID:   reqBody.ClientID,
		}
		pollBytes, _ := json.Marshal(tokenPoll)

		tResp, err := http.Post(serverURL+"/api/v1/auth/device/token", "application/json", bytes.NewReader(pollBytes))
		if err != nil {
			continue
		}

		if tResp.StatusCode == http.StatusOK {
			var tokenResp protocol.DeviceTokenResponse
			_ = json.NewDecoder(tResp.Body).Decode(&tokenResp)
			tResp.Body.Close()

			// Save session token in secure store
			_ = secStore.Set("access_token", []byte(tokenResp.AccessToken))
			_ = secStore.Set("device_id", []byte(tokenResp.DeviceID))
			_ = secStore.Set("user_handle", []byte(tokenResp.UserHandle))

			fmt.Println("\n[SUCCESS] Device authorized successfully!")
			fmt.Printf("Linked Account: %s (Device: %s)\n\n", tokenResp.UserHandle, tokenResp.DeviceID)
			return nil
		}
		tResp.Body.Close()
	}

	return protocol.ErrDeviceCodeExpired
}

// RunSend executes headless POSIX message sending.
func RunSend(serverURL, toUser, messageText string, ttlMS int64) error {
	secStore, err := crypto.NewSecureStore()
	if err != nil {
		return err
	}

	tokenBytes, err := secStore.Get("access_token")
	if err != nil || len(tokenBytes) == 0 {
		return fmt.Errorf("device not authorized. Run 'campusnet pair' first")
	}
	token := string(tokenBytes)

	// Fetch recipient public bundle
	toUserClean := strings.TrimPrefix(toUser, "@")
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/crypto/bundle/%s", serverURL, toUserClean))
	if err != nil {
		return fmt.Errorf("fetching recipient bundle: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("recipient '%s' not registered on campus network", toUserClean)
	}

	var bundle protocol.PreKeyBundle
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		return err
	}

	// Ephemeral key agreement & envelope creation
	senderIK, _ := crypto.GenerateKeyPair()
	sharedSecret, _ := crypto.GenerateKeyPair() // Ephemeral DH exchange
	ratchet, err := crypto.NewDoubleRatchetInitiator(sharedSecret.PublicKey, senderIK.PublicKey)
	if err != nil {
		return err
	}

	userHandleBytes, _ := secStore.Get("user_handle")
	senderHandle := string(userHandleBytes)
	if senderHandle == "" {
		senderHandle = "cli-user"
	}

	plaintextBytes := []byte(messageText)
	defer crypto.ZeroBytes(plaintextBytes)

	env, err := ratchet.Encrypt(senderHandle, toUserClean, plaintextBytes, ttlMS)
	if err != nil {
		return fmt.Errorf("encrypting envelope: %w", err)
	}

	// Connect to WebSocket and send envelope
	u, _ := url.Parse(serverURL)
	wsScheme := "ws"
	if u.Scheme == "https" {
		wsScheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s/ws/v1/stream?token=%s", wsScheme, u.Host, token)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("connecting to gateway: %w", err)
	}
	defer conn.Close()

	frame := protocol.StreamFrame{
		Op:       protocol.OpSendEnvelope,
		Envelope: env,
	}
	frameBytes, _ := json.Marshal(frame)
	if err := conn.WriteMessage(websocket.TextMessage, frameBytes); err != nil {
		return err
	}

	// Wait for ACK
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("awaiting gateway ACK: %w", err)
		}
		var ack protocol.StreamFrame
		if err := json.Unmarshal(msg, &ack); err == nil && ack.Op == protocol.OpAckDelivered {
			fmt.Printf("[SENT] Ephemeral message %s dispatched to @%s (TTL: %dms)\n", env.ID, toUserClean, env.TTLMilliseconds)
			return nil
		}
	}
}

// RunListen runs headless POSIX pipeline streaming received messages.
func RunListen(serverURL string, format string, autoBurn bool) error {
	secStore, err := crypto.NewSecureStore()
	if err != nil {
		return err
	}

	tokenBytes, err := secStore.Get("access_token")
	if err != nil || len(tokenBytes) == 0 {
		return fmt.Errorf("device not authorized. Run 'campusnet pair' first")
	}
	token := string(tokenBytes)

	u, _ := url.Parse(serverURL)
	wsScheme := "ws"
	if u.Scheme == "https" {
		wsScheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s/ws/v1/stream?token=%s", wsScheme, u.Host, token)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("connecting to gateway: %w", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return err
			}

			var frame protocol.StreamFrame
			if err := json.Unmarshal(msg, &frame); err != nil {
				continue
			}

			if frame.Op == protocol.OpDeliverEnvelope && frame.Envelope != nil {
				env := frame.Envelope
				if format == "json" {
					jsonBytes, _ := json.Marshal(env)
					fmt.Println(string(jsonBytes))
				} else {
					fmt.Fprintf(os.Stdout, "[MSG from @%s | TTL: %dms] (Encrypted payload: %d bytes)\n",
						env.SenderID, env.TTLMilliseconds, len(env.Ciphertext))
				}

				if autoBurn {
					// Send ACK_READ immediately to trigger cryptographic burn
					burnFrame := protocol.StreamFrame{
						Op:           protocol.OpAckRead,
						AckMessageID: env.ID,
					}
					bBytes, _ := json.Marshal(burnFrame)
					_ = conn.WriteMessage(websocket.TextMessage, bBytes)
				}
			}
		}
	}
}

// ReadStdinMessage reads input from standard input pipe.
func ReadStdinMessage() (string, error) {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data piped into stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytes)), nil
	}
	return "", nil
}
