package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuditSessionReferencesExpireAndDifferFromCredentials(t *testing.T) {
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Setup("long-test-password"); err != nil {
		t.Fatal(err)
	}
	token, ok := m.Login("long-test-password", "127.0.0.1:1")
	if !ok {
		t.Fatal("login")
	}
	id := m.AuditIDForToken(token)
	if len(id) != 32 || strings.Contains(token, id) {
		t.Fatal("bad reference")
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	if m.AuditSessionID(r) != id {
		t.Fatal("missing session reference")
	}
	second, ok := m.Login("long-test-password", "127.0.0.1:1")
	if !ok || m.AuditIDForToken(second) == id {
		t.Fatal("reference reused")
	}
	m.mu.Lock()
	s := m.sessions[token]
	s.Expires = time.Now().Add(-time.Second)
	m.sessions[token] = s
	m.mu.Unlock()
	if m.AuditSessionID(r) != "" || m.AuditIDForToken("unknown") != "" {
		t.Fatal("invalid session identified")
	}
}
