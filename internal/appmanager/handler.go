package appmanager

import (
	"encoding/json"
	"errors"
	"github.com/p8reit/LocasOS/internal/auditop"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Request struct {
	App   string              `json:"app"`
	Audit auditop.Attribution `json:"audit"`
}

func (r Request) Validate() error {
	if r.App != "ollama" && r.App != "open-webui" {
		return errors.New("unsupported application")
	}
	return r.Audit.Validate(true)
}

type Manager struct {
	Dir string
	mu  sync.Mutex
}

func New(dir string) *Manager { return &Manager{Dir: dir} }

func (m *Manager) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", m.status)
	mux.HandleFunc("POST /install", m.install)
	return mux
}

func readStatus(path string) string {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "idle"
	}
	s := strings.TrimSpace(string(bytes))
	if len(s) > 100 {
		return "unknown"
	}
	return s
}

func (m *Manager) status(w http.ResponseWriter, r *http.Request) {
	status := readStatus("/var/lib/locas-apps/app-install.status")
	_, ollamaErr := os.Stat("/usr/local/bin/ollama")
	_, webuiErr := os.Stat("/var/lib/locas-open-webui/data")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"job":                    status,
		"ollama_installed":       ollamaErr == nil,
		"workspace_data_present": webuiErr == nil,
	})
}

func (m *Manager) install(w http.ResponseWriter, r *http.Request) {
	state := auditop.From(r.Context())
	peer, observed := auditop.Peer(r.Context())
	if state == nil || !observed || state.Attribution.Validate(true) != nil || *state.Attribution.PeerUID != peer {
		http.Error(w, "trusted operation context required", http.StatusForbidden)
		return
	}

	var body struct {
		App string `json:"app"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	if dec.Decode(new(any)) != io.EOF {
		http.Error(w, "invalid request", 400)
		return
	}
	if body.App != "ollama" && body.App != "open-webui" {
		http.Error(w, "unsupported application", 400)
		return
	}
	state.App = body.App
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := os.MkdirAll(m.Dir, 0750); err != nil {
		http.Error(w, "state unavailable", 503)
		return
	}
	request := filepath.Join(m.Dir, "app-install.request")
	statusPath := "/var/lib/locas-apps/app-install.status"
	if data, err := os.ReadFile(statusPath); err == nil && strings.HasPrefix(string(data), "installing:") {
		http.Error(w, "installation already running", 409)
		return
	}
	temp := filepath.Join(m.Dir, "app-install.queue.tmp")
	f, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		code := http.StatusServiceUnavailable
		if errors.Is(err, os.ErrExist) {
			code = http.StatusConflict
		}
		http.Error(w, "installation already queued or unavailable", code)
		return
	}
	writeErr := json.NewEncoder(f).Encode(Request{App: body.App, Audit: state.Attribution})
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(temp)
		http.Error(w, "queue failed", 503)
		return
	}
	if err := os.Link(temp, request); err != nil {
		_ = os.Remove(temp)
		code := http.StatusServiceUnavailable
		if errors.Is(err, os.ErrExist) {
			code = http.StatusConflict
		}
		http.Error(w, "installation already queued or unavailable", code)
		return
	}
	_ = os.Remove(temp)
	directory, err := os.Open(m.Dir)
	if err != nil {
		state.Outcome = "uncertain"
		http.Error(w, "queue durability unavailable", 503)
		return
	}
	syncErr := directory.Sync()
	closeErr = directory.Close()
	if syncErr != nil || closeErr != nil {
		state.Outcome = "uncertain"
		http.Error(w, "queue durability unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "queued", "app": body.App})
}
