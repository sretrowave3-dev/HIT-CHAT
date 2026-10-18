package transport

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hit-chat/campusnet/internal/auth"
	"github.com/hit-chat/campusnet/internal/ephemeral"
	"github.com/hit-chat/campusnet/internal/geo"
	"github.com/hit-chat/campusnet/pkg/protocol"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all client surfaces (Web SPA, Tauri Desktop, CLI)
	},
}

// Client represents a single active WebSocket connection.
type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
	deviceID   string
	userHandle string
}

// Hub manages active WebSocket connections, message routing, and presence broadcasts.
type Hub struct {
	mu            sync.RWMutex
	clients       map[*Client]bool
	byDevice      map[string]*Client
	byUser        map[string][]*Client
	register      chan *Client
	unregister    chan *Client
	ephemeralMgr  *ephemeral.Manager
	privacyFilter *geo.ZeroBypassPrivacyFilter
}

// NewHub initializes the WebSocket hub.
func NewHub(ephemMgr *ephemeral.Manager, filter *geo.ZeroBypassPrivacyFilter) *Hub {
	return &Hub{
		clients:       make(map[*Client]bool),
		byDevice:      make(map[string]*Client),
		byUser:        make(map[string][]*Client),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		ephemeralMgr:  ephemMgr,
		privacyFilter: filter,
	}
}

// Run starts the WebSocket event multiplexer loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.byDevice[client.deviceID] = client
			h.byUser[client.userHandle] = append(h.byUser[client.userHandle], client)
			h.mu.Unlock()
			log.Printf("[WS HUB] Connected device: %s (user: %s)", client.deviceID, client.userHandle)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.byDevice, client.deviceID)

				// Remove from user slice
				userList := h.byUser[client.userHandle]
				var updated []*Client
				for _, c := range userList {
					if c != client {
						updated = append(updated, c)
					}
				}
				if len(updated) == 0 {
					delete(h.byUser, client.userHandle)
				} else {
					h.byUser[client.userHandle] = updated
				}

				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[WS HUB] Disconnected device: %s", client.deviceID)
		}
	}
}

// ServeWS handles WebSocket connection upgrade and authentication.
func (h *Hub) ServeWS(tokenMgr *auth.TokenManager, w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) > 7 {
			tokenStr = authHeader[7:]
		}
	}

	claims, err := tokenMgr.ValidateToken(tokenStr)
	if err != nil {
		http.Error(w, "unauthorized WebSocket upgrade: invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:        h,
		conn:       conn,
		send:       make(chan []byte, 256),
		deviceID:   claims.DeviceID,
		userHandle: claims.UserHandle,
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(1048576) // 1MB max message frame
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var frame protocol.StreamFrame
		if err := json.Unmarshal(message, &frame); err != nil {
			log.Printf("[WS] Invalid frame JSON: %v", err)
			continue
		}

		c.handleFrame(&frame)
	}
}

func (c *Client) handleFrame(frame *protocol.StreamFrame) {
	ctx := context.Background()

	switch frame.Op {
	case protocol.OpSendEnvelope:
		if frame.Envelope == nil {
			return
		}
		// Validate envelope dimensions
		if err := frame.Envelope.Validate(); err != nil {
			c.sendError("invalid envelope: " + err.Error())
			return
		}

		// Ensure sender ID matches authenticated claims
		frame.Envelope.SenderID = c.userHandle

		// Save in ephemeral store with TTL
		if err := c.hub.ephemeralMgr.SaveEnvelope(ctx, frame.Envelope); err != nil {
			c.sendError("failed to store ephemeral envelope: " + err.Error())
			return
		}

		// Check if recipient is connected
		delivered := c.hub.routeEnvelope(frame.Envelope)

		// Send ACK back to sender
		ackFrame := &protocol.StreamFrame{
			Op:           protocol.OpAckDelivered,
			AckMessageID: frame.Envelope.ID,
		}
		c.sendFrame(ackFrame)
		log.Printf("[WS] Routed envelope %s to %s (online=%v)", frame.Envelope.ID, frame.Envelope.RecipientID, delivered)

	case protocol.OpAckRead:
		if frame.AckMessageID != "" {
			// Trigger immediate burn-on-read in ephemeral store
			_ = c.hub.ephemeralMgr.BurnOnRead(ctx, frame.AckMessageID)
			log.Printf("[WS] Envelope burned on read: %s", frame.AckMessageID)
		}

	case protocol.OpUpdatePresence:
		// Scrub and sanitize location telemetry according to privacy switch
		sanitized := c.hub.privacyFilter.SanitizePresence(c.userHandle, frame.Presence)
		broadcastFrame := &protocol.StreamFrame{
			Op:       protocol.OpZonePresence,
			Presence: sanitized,
		}
		c.hub.broadcastToAll(broadcastFrame)

	case protocol.OpHeartbeat:
		pongFrame := &protocol.StreamFrame{
			Op: protocol.OpHeartbeat,
		}
		c.sendFrame(pongFrame)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) sendFrame(frame *protocol.StreamFrame) {
	data, err := json.Marshal(frame)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
		log.Printf("[WS] Buffer full, dropped frame for device %s", c.deviceID)
	}
}

func (c *Client) sendError(msg string) {
	c.sendFrame(&protocol.StreamFrame{
		Op:    protocol.OpError,
		Error: msg,
	})
}

func (h *Hub) routeEnvelope(env *protocol.Envelope) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	frame := &protocol.StreamFrame{
		Op:       protocol.OpDeliverEnvelope,
		Envelope: env,
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return false
	}

	delivered := false

	// Try routing by deviceID directly
	if client, ok := h.byDevice[env.RecipientID]; ok {
		select {
		case client.send <- data:
			delivered = true
		default:
		}
	}

	// Also try routing by userHandle
	if clients, ok := h.byUser[env.RecipientID]; ok {
		for _, client := range clients {
			select {
			case client.send <- data:
				delivered = true
			default:
			}
		}
	}

	return delivered
}

func (h *Hub) broadcastToAll(frame *protocol.StreamFrame) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, err := json.Marshal(frame)
	if err != nil {
		return
	}

	for client := range h.clients {
		select {
		case client.send <- data:
		default:
		}
	}
}
