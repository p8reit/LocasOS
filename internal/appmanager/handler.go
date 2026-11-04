package appmanager

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/p8reit/LocasOS/internal/auditop"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	Dir           string
	StatusPath    string
	BinaryPath    string
	WorkspacePath string
	Ready         func(context.Context) bool
	mu            sync.Mutex
}

func New(dir string) *Manager {
	return &Manager{Dir: dir, StatusPath: "/var/lib/locas-apps/app-install.status", BinaryPath: "/usr/local/bin/ollama", WorkspacePath: "/var/lib/locas-open-webui/data", Ready: RuntimeReady}
}

func (m *Manager) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", m.status)
	mux.HandleFunc("POST /install", m.install)
	return mux
}

func (m *Manager) status(w http.ResponseWriter, r *http.Request) {
	status := ReadStatus(m.StatusPath)
	binary, ollamaErr := os.Stat(m.BinaryPath)
	binaryPresent := ollamaErr == nil && binary.Mode().IsRegular() && binary.Mode().Perm()&0111 != 0
	workspace, webuiErr := os.Stat(m.WorkspacePath)
	workspaceState := "unknown"
	if webuiErr == nil && workspace.IsDir() {
		workspaceState = "present"
	} else if os.IsNotExist(webuiErr) {
		workspaceState = "absent"
	}
	ready := m.Ready != nil && m.Ready(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"job": status.Job, "state": status.State, "app": status.App, "reason": status.Reason, "correlation_id": status.CorrelationID, "status_token": status.StatusToken, "ollama_installed": binaryPresent, "ollama_ready": ready, "workspace_data_present": workspaceState == "present", "workspace_state": workspaceState})
}

func (m *Manager) install(w http.ResponseWriter, r *http.Request) {
	state := auditop.From(r.Context())
	peer, observed := auditop.Peer(r.Context())
	if state == nil || !observed || state.Attribution.Validate(true) != nil || *state.Attribution.PeerUID != peer {
		http.Error(w, "trusted operation context required", http.StatusForbidden)
		return
	}

	var body struct {
		App               string `json:"app"`
		AcknowledgeStatus string `json:"acknowledge_status,omitempty"`
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
	if !ValidAcknowledgement(body.AcknowledgeStatus) {
		http.Error(w, "invalid status acknowledgement", 400)
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
	current := ReadStatus(m.StatusPath)
	if current.State == "installing" {
		http.Error(w, "installation already running", 409)
		return
	}
	if current.State == "unknown" {
		http.Error(w, "installation status unavailable; inspect worker state before retrying", 503)
		return
	}
	if current.State == "uncertain" && body.AcknowledgeStatus != current.StatusToken {
		http.Error(w, "previous installation outcome uncertain; inspect and acknowledge its current status before retrying", 409)
		return
	}
	if body.App == "open-webui" && (m.Ready == nil || !m.Ready(r.Context())) {
		http.Error(w, "Ollama service/API is not ready; inspect Ollama before installing Open WebUI", 409)
		return
	}
	if ReadStatus(m.StatusPath) != current {
		http.Error(w, "installer status changed; refresh before retrying", 409)
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
	if ReadStatus(m.StatusPath) != current {
		_ = os.Remove(temp)
		http.Error(w, "installer status changed; refresh before retrying", 409)
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
