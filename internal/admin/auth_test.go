package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFirstRunAndSession(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Configured() {
		t.Fatal("new appliance must not be configured")
	}
	if err := m.Setup("this-is-a-long-and-unique-password"); err != nil {
		t.Fatal(err)
	}
	if !m.Configured() {
		t.Fatal("setup not persisted")
	}
	if err := m.Setup("different-very-long-password"); err == nil {
		t.Fatal("administrator setup must be one-time")
	}
	m, err = New(dir)
	if err != nil || !m.Configured() {
		t.Fatalf("reloaded admin state: %v", err)
	}
	token, ok := m.Login("this-is-a-long-and-unique-password", "127.0.0.1:55555")
	if !ok || token == "" {
		t.Fatal("valid login failed")
	}
	w := httptest.NewRecorder()
	SetCookie(w, token, false)
	req := httptest.NewRequest(http.MethodGet, "/api/telemetry", nil)
	req.AddCookie(w.Result().Cookies()[0])
	if !m.Authenticated(req) {
		t.Fatal("cookie session not accepted")
	}
	m.Logout(req)
	if m.Authenticated(req) {
		t.Fatal("logout did not invalidate session")
	}
}

func TestMutationCSRFGuard(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:59420/api/login", nil)
	req.Header.Set("Content-Type", "application/json")
	if SafeMutation(req) {
		t.Fatal("custom header must be required")
	}
	req.Header.Set("X-Locas-Request", "1")
	req.Header.Set("Origin", "http://evil.example")
	if SafeMutation(req) {
		t.Fatal("cross-origin request must be rejected")
	}
	req.Header.Set("Origin", "http://127.0.0.1:59420")
	if !SafeMutation(req) {
		t.Fatal("same-origin JSON request should be allowed")
	}
}
