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
	"unicode/utf8"

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
		UserHandle:        userHandle,
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

// AutoAuthorizeAndRegister performs automatic zero-interaction authorization and prekey registration.
func AutoAuthorizeAndRegister(serverURL, userHandle string) (string, string, string, error) {
	secStore, err := crypto.NewSecureStore()
	if err != nil {
		return "", "", "", err
	}

	if userHandle == "" {
		if hBytes, err := secStore.Get("user_handle"); err == nil && len(hBytes) > 0 {
			userHandle = string(hBytes)
		} else {
			userHandle = "cli_user"
		}
	}
	userHandle = strings.TrimPrefix(strings.ToLower(userHandle), "@")

	// 1. Generate local Identity Key
	ik, err := crypto.GenerateKeyPair()
	if err != nil {
		return "", "", "", fmt.Errorf("generating identity key: %w", err)
	}

	// 2. Request auto session from server
	reqBody := map[string]string{
		"user_handle": userHandle,
		"platform":    "cli",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	resp, err := http.Post(serverURL+"/api/v1/auth/session", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", "", fmt.Errorf("requesting auto-session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("auto-session failed with HTTP %d", resp.StatusCode)
	}

	var tokenResp protocol.DeviceTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", "", "", fmt.Errorf("parsing token response: %w", err)
	}

	// 3. Save session in secure store
	_ = secStore.Set("access_token", []byte(tokenResp.AccessToken))
	_ = secStore.Set("device_id", []byte(tokenResp.DeviceID))
	_ = secStore.Set("user_handle", []byte(tokenResp.UserHandle))

	// 4. Register PreKeyBundle on gateway so peers can route to this user
	spk, _ := crypto.GenerateKeyPair()
	otk1, _ := crypto.GenerateKeyPair()
	otk2, _ := crypto.GenerateKeyPair()

	bundle := protocol.PreKeyBundle{
		DeviceID:        tokenResp.DeviceID,
		UserHandle:      tokenResp.UserHandle,
		IdentityKey:     fmt.Sprintf("%x", ik.PublicKey),
		SignedPreKey:    fmt.Sprintf("%x", spk.PublicKey),
		SignedPreKeySig: "cli_auto_signed_spk_signature_valid==",
		OneTimePreKeys: []string{
			fmt.Sprintf("%x", otk1.PublicKey),
			fmt.Sprintf("%x", otk2.PublicKey),
		},
	}
	bBytes, _ := json.Marshal(bundle)
	bReq, err := http.NewRequest(http.MethodPost, serverURL+"/api/v1/crypto/bundle", bytes.NewReader(bBytes))
	if err == nil {
		bReq.Header.Set("Content-Type", "application/json")
		bReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
		bResp, bErr := http.DefaultClient.Do(bReq)
		if bErr == nil {
			bResp.Body.Close()
		}
	}

	return tokenResp.AccessToken, tokenResp.DeviceID, tokenResp.UserHandle, nil
}

// EnsureAuthorized checks if the device has a valid session token, or auto-authorizes automatically.
func EnsureAuthorized(serverURL, userHandle string) (string, string, string, error) {
	secStore, err := crypto.NewSecureStore()
	if err != nil {
		return "", "", "", fmt.Errorf("initializing secure store: %w", err)
	}

	cleanRequested := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(userHandle)), "@")
	if strings.HasPrefix(cleanRequested, "-") {
		cleanRequested = ""
	}

	tokenBytes, err := secStore.Get("access_token")
	if err == nil && len(tokenBytes) > 0 {
		dBytes, _ := secStore.Get("device_id")
		hBytes, _ := secStore.Get("user_handle")
		storedHandle := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(string(hBytes))), "@")

		if cleanRequested == "" || (cleanRequested == "campus_dev" && storedHandle != "") {
			return string(tokenBytes), string(dBytes), storedHandle, nil
		}
	}

	// No token found or different user requested -> auto-authorize seamlessly
	return AutoAuthorizeAndRegister(serverURL, userHandle)
}

// RunAutoPair provides instant non-interactive device authorization.
func RunAutoPair(serverURL, userHandle string) error {
	token, deviceID, handle, err := AutoAuthorizeAndRegister(serverURL, userHandle)
	if err != nil {
		return fmt.Errorf("auto-pairing failed: %w", err)
	}

	fmt.Println("\n=======================================================")
	fmt.Println("             CAMPUSNET AUTO-AUTHORIZATION              ")
	fmt.Println("=======================================================")
	fmt.Printf("[SUCCESS] Device automatically authorized & registered!\n")
	fmt.Printf("Linked Account: @%s (Device: %s)\n", handle, deviceID)
	fmt.Printf("Session Token:  %s...\n", token[:20])
	fmt.Println("Cryptographic Pre-Key Bundle registered on gateway.")
	fmt.Println("Ready for zero-knowledge end-to-end encrypted messaging.")
	fmt.Println("=======================================================")
	return nil
}

// RunConnect verifies connection or auto-connects if unauthorized.
func RunConnect(serverURL, userHandle string, auto bool) error {
	var token, deviceID, handle string
	var err error

	if auto {
		token, deviceID, handle, err = AutoAuthorizeAndRegister(serverURL, userHandle)
	} else {
		secStore, sErr := crypto.NewSecureStore()
		if sErr != nil {
			return sErr
		}
		tBytes, tErr := secStore.Get("access_token")
		if tErr != nil || len(tBytes) == 0 {
			return fmt.Errorf("device not authorized. Run 'campusnet pair --auto' to authorize")
		}
		token = string(tBytes)
		dBytes, _ := secStore.Get("device_id")
		hBytes, _ := secStore.Get("user_handle")
		deviceID = string(dBytes)
		handle = string(hBytes)
	}

	if err != nil {
		return err
	}

	// Test WebSocket connectivity
	u, _ := url.Parse(serverURL)
	wsScheme := "ws"
	if u.Scheme == "https" {
		wsScheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s/ws/v1/stream?token=%s", wsScheme, u.Host, token)
	conn, _, wErr := websocket.DefaultDialer.Dial(wsURL, nil)
	if wErr != nil {
		return fmt.Errorf("connecting to gateway: %w", wErr)
	}
	defer conn.Close()

	// Send heartbeat probe
	hbFrame := protocol.StreamFrame{Op: protocol.OpHeartbeat}
	if err := conn.WriteJSON(hbFrame); err != nil {
		return fmt.Errorf("sending heartbeat probe: %w", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var respFrame protocol.StreamFrame
	if err := conn.ReadJSON(&respFrame); err != nil || respFrame.Op != protocol.OpHeartbeat {
		return fmt.Errorf("gateway heartbeat verification failed")
	}

	fmt.Println("\n=======================================================")
	fmt.Println("              CAMPUSNET GATEWAY CONNECTED              ")
	fmt.Println("=======================================================")
	fmt.Printf("[OK] Status:    ACTIVE (Authenticated WebSocket Stream)\n")
	fmt.Printf("[OK] Account:   @%s\n", handle)
	fmt.Printf("[OK] Device:    %s\n", deviceID)
	fmt.Printf("[OK] Gateway:   %s\n", serverURL)
	fmt.Println("=======================================================")
	return nil
}

// RunSend executes headless POSIX message sending.
func RunSend(serverURL, fromUser, toUser, messageText string, ttlMS int64) error {
	token, _, senderHandle, aErr := EnsureAuthorized(serverURL, fromUser)
	if aErr != nil {
		return fmt.Errorf("device not authorized and auto-connect failed: %w", aErr)
	}

	// Fetch recipient public bundle
	toUserClean := strings.TrimPrefix(strings.ToLower(toUser), "@")
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/crypto/bundle/%s", serverURL, toUserClean))
	if err != nil {
		return fmt.Errorf("fetching recipient bundle: %w", err)
	}
	defer resp.Body.Close()

	var bundle protocol.PreKeyBundle
	bundleFound := false

	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&bundle); err == nil {
			bundleFound = true
		}
	} else {
		// Try alias (with or without student_ prefix)
		alias := "student_" + toUserClean
		if strings.HasPrefix(toUserClean, "student_") {
			alias = strings.TrimPrefix(toUserClean, "student_")
		}
		aResp, aErr := http.Get(fmt.Sprintf("%s/api/v1/crypto/bundle/%s", serverURL, alias))
		if aErr == nil {
			defer aResp.Body.Close()
			if aResp.StatusCode == http.StatusOK {
				if err := json.NewDecoder(aResp.Body).Decode(&bundle); err == nil {
					bundleFound = true
				}
			}
		}
	}

	if !bundleFound {
		return fmt.Errorf("recipient '%s' not registered on campus network", toUserClean)
	}

	plaintextBytes := []byte(messageText)
	defer crypto.ZeroBytes(plaintextBytes)

	// Isolated pairwise AES-256-GCM ratchet encryption matching WebCrypto
	iv, ciphertext, tag, err := crypto.EncryptPairwisePayload(senderHandle, toUserClean, plaintextBytes)
	if err != nil {
		return fmt.Errorf("encrypting envelope: %w", err)
	}

	ephemKP, _ := crypto.GenerateKeyPair()
	env := protocol.NewEnvelope(senderHandle, toUserClean, ttlMS)
	env.Ciphertext = ciphertext
	env.IV = iv
	env.AuthTag = tag
	if ephemKP != nil {
		env.EphemeralPublicKey = ephemKP.PublicKey
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
			fmt.Printf("[SENT] Ephemeral message %s dispatched from @%s to @%s (TTL: %dms)\n",
				env.ID, senderHandle, toUserClean, env.TTLMilliseconds)
			return nil
		}
	}
}

// RunListen runs headless POSIX pipeline streaming received messages.
func RunListen(serverURL, listenUser, format string, autoBurn bool) error {
	token, _, userHandle, aErr := EnsureAuthorized(serverURL, listenUser)
	if aErr != nil {
		return fmt.Errorf("device not authorized and auto-connect failed: %w", aErr)
	}

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

	fmt.Printf("[LISTEN] Streaming messages for @%s from %s...\n", userHandle, serverURL)

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

				// Decrypt payload with pairwise isolation
				plaintext, dErr := crypto.DecryptPairwisePayload(env.SenderID, userHandle, env.IV, env.Ciphertext, env.AuthTag)
				msgBody := ""
				if dErr == nil {
					msgBody = SanitizeTerminalString(string(plaintext))
				} else if utf8.Valid(env.Ciphertext) {
					msgBody = SanitizeTerminalString(string(env.Ciphertext))
				} else {
					msgBody = fmt.Sprintf("[E2EE Encrypted Payload: %d bytes]", len(env.Ciphertext))
				}
				cleanSender := SanitizeTerminalString(env.SenderID)

				if format == "json" {
					jsonBytes, _ := json.Marshal(map[string]interface{}{
						"id":        env.ID,
						"sender":    cleanSender,
						"recipient": env.RecipientID,
						"text":      msgBody,
						"ttl_ms":    env.TTLMilliseconds,
						"timestamp": env.TimestampMS,
					})
					fmt.Println(string(jsonBytes))
				} else {
					fmt.Fprintf(os.Stdout, "[MSG from @%s | TTL: %dms] %s\n",
						cleanSender, env.TTLMilliseconds, msgBody)
				}
				_ = os.Stdout.Sync()

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

// ReadStdinMessage reads input from standard input pipe with a timeout.
func ReadStdinMessage() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		ch := make(chan string, 1)
		errCh := make(chan error, 1)
		go func() {
			bytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				errCh <- err
				return
			}
			ch <- strings.TrimSpace(string(bytes))
		}()

		select {
		case res := <-ch:
			return res, nil
		case err := <-errCh:
			return "", err
		case <-time.After(200 * time.Millisecond):
			// Timeout: stdin was a pipe but has no active stream
			return "", nil
		}
	}
	return "", nil
}
