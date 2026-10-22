package updatequeue

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectUntrustedUpdateRequest(t *testing.T) {
	m := New(t.TempDir())
	for _, body := range []string{
		`{"channel":"dev","version":"v0.10.0"}`,
		`{"channel":"stable","version":"../shell"}`,
		`{"channel":"stable","version":"v0.10.0","url":"https://evil.invalid"}`,
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/install", strings.NewReader(body))
		m.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected bad request for %s, got %d", body, w.Code)
		}
	}
}

func TestQueueCannotReplaceExistingRequest(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	body := `{"channel":"beta","version":"v0.10.0"}`
	for _, expected := range []int{http.StatusAccepted, http.StatusConflict} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/install", strings.NewReader(body))
		m.Handler().ServeHTTP(w, r)
		if w.Code != expected {
			t.Fatalf("expected %d got %d", expected, w.Code)
		}
	}
	content, err := os.ReadFile(filepath.Join(dir, "update-install.request"))
	if err != nil || !strings.Contains(string(content), "v0.10.0") {
		t.Fatalf("unexpected queued request %s: %v", content, err)
	}
}
