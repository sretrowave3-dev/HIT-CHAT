package transport

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

// MetricsTracker maintains atomic, zero-knowledge server observability counters.
type MetricsTracker struct {
	Sent            atomic.Uint64 `json:"messages_sent"`
	Delivered       atomic.Uint64 `json:"messages_delivered"`
	Nacks           atomic.Uint64 `json:"nacks_received"`
	DecryptFailures atomic.Uint64 `json:"decrypt_failures"`
	Reconnects      atomic.Uint64 `json:"reconnects"`
}

// GlobalMetrics holds the system-wide metrics instance.
var GlobalMetrics = &MetricsTracker{}

// Snapshot returns a copy of current metrics values.
func (m *MetricsTracker) Snapshot() map[string]uint64 {
	return map[string]uint64{
		"messages_sent":    m.Sent.Load(),
		"messages_delivered": m.Delivered.Load(),
		"nacks_received":   m.Nacks.Load(),
		"decrypt_failures": m.DecryptFailures.Load(),
		"reconnects":       m.Reconnects.Load(),
	}
}

// HandleMetrics serves the JSON metrics endpoint for telemetry monitoring.
func (r *Router) handleMetrics(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(GlobalMetrics.Snapshot())
}
