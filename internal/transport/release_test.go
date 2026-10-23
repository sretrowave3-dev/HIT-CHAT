//go:build !devauth

package transport

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestReleaseBuildDevAuthReturns404 proves that in standard production/release builds,
// the /api/v1/dev/login endpoint does not exist and strictly returns 404 Not Found.
func TestReleaseBuildDevAuthReturns404(t *testing.T) {
	router, _, _ := setupTestRouter()
	router.SetDevAuthEnabled(true) // Even if flag was passed, endpoint must not exist

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dev/login", bytes.NewBufferString(`{"handle":"alice"}`))
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("SECURITY VIOLATION: release build returned %d for dev login endpoint (expected 404 Not Found)", rec.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/dev/login", nil)
	getReq.RemoteAddr = "127.0.0.1:54321"
	getRec := httptest.NewRecorder()

	router.Handler().ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusNotFound {
		t.Fatalf("SECURITY VIOLATION: release build returned %d for GET dev login (expected 404 Not Found)", getRec.Code)
	}
}
