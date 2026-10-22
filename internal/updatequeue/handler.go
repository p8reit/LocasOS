package updatequeue

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

var allowedVersion = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

type Manager struct {
	Dir string
	mu sync.Mutex
}

type Request struct {
	Channel string `json:"channel"`
	Version string `json:"version"`
}

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
	var req Request
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid update approval", http.StatusBadRequest)
		return
	}
	if (req.Channel != "stable" && req.Channel != "beta") || !allowedVersion.MatchString(req.Version) {
		http.Error(w, "unsupported update channel or version", http.StatusBadRequest)
		return
	}
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"state": "queued",
		"version": req.Version,
		"channel": req.Channel,
	})
}
