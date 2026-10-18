package protocol

// StreamOp represents the operation opcode in a WebSocket or TCP frame.
type StreamOp string

const (
	OpSendEnvelope     StreamOp = "SEND_ENVELOPE"
	OpDeliverEnvelope  StreamOp = "DELIVER_ENVELOPE"
	OpAckDelivered     StreamOp = "ACK_DELIVERED"
	OpAckRead          StreamOp = "ACK_READ"
	OpHeartbeat        StreamOp = "HEARTBEAT"
	OpUpdatePresence   StreamOp = "UPDATE_PRESENCE"
	OpZonePresence     StreamOp = "ZONE_PRESENCE"
	OpError            StreamOp = "ERROR"
)

// StreamFrame is the full-duplex wire message sent over WebSocket connections.
type StreamFrame struct {
	Op           StreamOp         `json:"op"`
	SessionID    string           `json:"session_id,omitempty"`
	Envelope     *Envelope        `json:"envelope,omitempty"`
	AckMessageID string           `json:"ack_message_id,omitempty"`
	Presence     *PresencePayload `json:"presence,omitempty"`
	Error        string           `json:"error,omitempty"`
}

// PresencePayload contains spatial presence data with privacy switch flags.
type PresencePayload struct {
	UserHandle      string   `json:"user_handle,omitempty"`
	BSSIDHash       string   `json:"bssid_hash,omitempty"`
	ZoneCode        string   `json:"zone_code,omitempty"`
	DisplayZone     string   `json:"display_zone,omitempty"`
	BuildingName    string   `json:"building_name,omitempty"`
	FloorLevel      int      `json:"floor_level,omitempty"`
	LocationEnabled bool     `json:"location_enabled"`
	Latitude        *float64 `json:"latitude,omitempty"`
	Longitude       *float64 `json:"longitude,omitempty"`
}

// PreKeyBundle models the public keys uploaded and exchanged for X3DH handshakes.
type PreKeyBundle struct {
	DeviceID             string   `json:"device_id"`
	UserHandle           string   `json:"user_handle"`
	IdentityKey          string   `json:"identity_key"`           // Base64 public X25519
	SignedPreKey         string   `json:"signed_prekey"`          // Base64 public X25519
	SignedPreKeySig      string   `json:"signed_prekey_sig"`      // Base64 signature
	OneTimePreKeys       []string `json:"one_time_prekeys,omitempty"` // Array of Base64 OTK public keys
	SelectedOneTimePreKey string  `json:"selected_otk,omitempty"` // Single consumed OTK returned by server
}

// DeviceCodeRequest is the payload sent by a client to initiate device login.
type DeviceCodeRequest struct {
	ClientID          string `json:"client_id"`
	Platform          string `json:"platform"` // "cli", "web", "desktop"
	PublicIdentityKey string `json:"public_identity_key"`
}

// DeviceCodeResponse provides authorization codes and instructions to the client.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"` // Seconds
	Interval        int    `json:"interval"`   // Polling interval in seconds
}

// DeviceTokenResponse is returned upon successful device authorization.
type DeviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	DeviceID    string `json:"device_id"`
	UserHandle  string `json:"user_handle"`
}
