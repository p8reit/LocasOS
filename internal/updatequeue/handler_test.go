package updatequeue

import (
	"encoding/json"
	"github.com/p8reit/LocasOS/internal/updateapproval"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	job := Request{Selection: updateapproval.Selection{Channel: "beta", Version: "v0.10.0", SHA256: strings.Repeat("a", 64), Token: strings.Repeat("b", 64)}, ExpiresAt: time.Now().Add(time.Minute)}
	payload, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	body := string(payload)
	for _, expected := range []int{http.StatusAccepted, http.StatusConflict} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/install", strings.NewReader(body))
		m.Handler().ServeHTTP(w, r)
		if w.Code != expected {
			t.Fatalf("expected %d got %d", expected, w.Code)
		}
	}
	content, err := os.ReadFile(filepath.Join(dir, "update-install.request"))
	var persisted Request
	if decodeErr := json.Unmarshal(content, &persisted); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if persisted.Selection != job.Selection || !persisted.ExpiresAt.Equal(job.ExpiresAt) {
		t.Fatal("artifact identity changed in queue")
	}
	if err != nil || !strings.Contains(string(content), "v0.10.0") {
		t.Fatalf("unexpected queued request %s: %v", content, err)
	}
}

func TestQueueRejectsIncompleteAndExpiredArtifactApproval(t *testing.T) {
	valid := Request{Selection: updateapproval.Selection{Channel: "stable", Version: "v1.2.3", SHA256: strings.Repeat("a", 64), Token: strings.Repeat("b", 64)}, ExpiresAt: time.Now().Add(time.Minute)}
	for _, scenario := range []string{"missing digest", "invalid digest", "missing token", "expired", "trailing JSON", "unknown field"} {
		t.Run(scenario, func(t *testing.T) {
			job := valid
			switch scenario {
			case "missing digest":
				job.SHA256 = ""
			case "invalid digest":
				job.SHA256 = "../path"
			case "missing token":
				job.Token = ""
			case "expired":
				job.ExpiresAt = time.Now().Add(-time.Second)
			}
			payload, _ := json.Marshal(job)
			if scenario == "trailing JSON" {
				payload = append(payload, []byte(` {}`)...)
			}
			if scenario == "unknown field" {
				payload = append(payload[:len(payload)-1], []byte(`,"url":"https://evil.invalid"}`)...)
			}
			dir := t.TempDir()
			w := httptest.NewRecorder()
			New(dir).Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/install", strings.NewReader(string(payload))))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d", w.Code)
			}
			if _, err := os.Stat(filepath.Join(dir, "update-install.request")); !os.IsNotExist(err) {
				t.Fatal("invalid job queued")
			}
		})
	}
}
