package appmanager

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Manager struct {
	Dir string
	mu sync.Mutex
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
	if err != nil { return "idle" }
	s := strings.TrimSpace(string(bytes))
	if len(s) > 100 { return "unknown" }
	return s
}

func (m *Manager) status(w http.ResponseWriter, r *http.Request) {
	status := readStatus("/var/lib/locas-apps/app-install.status")
	_, ollamaErr := os.Stat("/usr/local/bin/ollama")
	_, webuiErr := os.Stat("/var/lib/locas-open-webui/data")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"job": status,
		"ollama_installed": ollamaErr == nil,
		"workspace_data_present": webuiErr == nil,
	})
}

func (m *Manager) install(w http.ResponseWriter, r *http.Request) {
	var body struct { App string `json:"app"` }
	dec := json.NewDecoder(http.MaxBytesReader(w,r.Body,1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil { http.Error(w,"invalid request",400);return }
	if body.App != "ollama" && body.App != "open-webui" { http.Error(w,"unsupported application",400);return }
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := os.MkdirAll(m.Dir,0750); err != nil { http.Error(w,"state unavailable",503);return }
	request := filepath.Join(m.Dir,"app-install.request")
	statusPath := "/var/lib/locas-apps/app-install.status"
	if data, err := os.ReadFile(statusPath); err == nil && strings.HasPrefix(string(data),"installing:") {
		http.Error(w,"installation already running",409);return
	}
	f, err := os.OpenFile(request,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0640)
	if err != nil {
		code:=http.StatusServiceUnavailable
		if errors.Is(err,os.ErrExist){code=http.StatusConflict}
		http.Error(w,"installation already queued or unavailable",code);return
	}
	_, writeErr := f.WriteString(body.App+"\n")
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil { _=os.Remove(request);http.Error(w,"queue failed",503);return }
	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status":"queued","app":body.App})
}
