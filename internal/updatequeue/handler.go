package updatequeue

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/p8reit/LocasOS/internal/auditop"
	"github.com/p8reit/LocasOS/internal/updateapproval"
)

type Manager struct {
	Dir string
	mu  sync.Mutex
}

type Request = updateapproval.Job

func New(dir string) *Manager {
	return &Manager{Dir: dir}
}

func (m *Manager) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", m.status)
	mux.HandleFunc("POST /install", m.install)
	return mux
}

func (m *Manager) status(w http.ResponseWriter, r *http.Request) {
	payload, err := os.ReadFile("/var/lib/locas-updates/status.json")
	if err != nil || len(payload) > 4096 || !json.Valid(payload) {
		payload = []byte(`{"state":"idle"}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(payload)
}

func (m *Manager) install(w http.ResponseWriter, r *http.Request) {
	state := auditop.From(r.Context())
	peer, observed := auditop.Peer(r.Context())
	if state == nil || !observed || state.Attribution.Validate(true) != nil || *state.Attribution.PeerUID != peer {
		http.Error(w, "trusted operation context required", http.StatusForbidden)
		return
	}

	var req Request
	if err := updateapproval.Decode(http.MaxBytesReader(w, r.Body, 1024), &req); err != nil || req.Validate(time.Now()) != nil {
		http.Error(w, "invalid or expired artifact approval", http.StatusBadRequest)
		return
	}
	if !req.Audit.SameAssertion(state.Attribution) {
		http.Error(w, "operation attribution mismatch", http.StatusForbidden)
		return
	}
	req.Audit = state.Attribution
	state.Version = req.Version
	state.Channel = req.Channel
	state.SHA256 = req.SHA256
	m.mu.Lock()
	defer m.mu.Unlock()
	status, _ := os.ReadFile("/var/lib/locas-updates/status.json")
	var prior struct {
		State string `json:"state"`
	}
	_ = json.Unmarshal(status, &prior)
	switch prior.State {
	case "checking", "downloading", "verifying", "applying":
		http.Error(w, "an update is already running", http.StatusConflict)
		return
	}
	if err := os.MkdirAll(m.Dir, 0750); err != nil {
		http.Error(w, "update queue unavailable", http.StatusServiceUnavailable)
		return
	}
	queued := filepath.Join(m.Dir, "update-install.request")
	temp := filepath.Join(m.Dir, "update-install.tmp")
	f, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		http.Error(w, "update request already pending", http.StatusConflict)
		return
	}
	writeErr := json.NewEncoder(f).Encode(req)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(temp)
		http.Error(w, "could not queue update", http.StatusServiceUnavailable)
		return
	}
	if err := os.Link(temp, queued); err != nil {
		_ = os.Remove(temp)
		code := http.StatusServiceUnavailable
		if errors.Is(err, os.ErrExist) {
			code = http.StatusConflict
		}
		http.Error(w, "update request already pending or unavailable", code)
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
	_ = json.NewEncoder(w).Encode(map[string]string{
		"state":   "queued",
		"version": req.Version,
		"channel": req.Channel,
		"sha256":  req.SHA256,
	})
}
