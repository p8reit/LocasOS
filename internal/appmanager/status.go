package appmanager

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"regexp"
	"strings"
	"syscall"
)

var referencePattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var acknowledgementPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// These codes contain no subprocess output, paths, credentials or request bodies.
func ValidReason(app, reason string) bool {
	switch reason {
	case "audit-outcome", "queue-durability", "installer-failed":
		return app == "ollama" || app == "open-webui"
	case "ollama-port-conflict", "ollama-preflight", "ollama-download", "ollama-archive", "ollama-extract", "ollama-service", "ollama-readiness":
		return app == "ollama"
	case "webui-runtime-config", "ollama-required", "webui-preflight", "webui-docker-install", "webui-docker-service", "webui-storage", "webui-pull", "webui-container-start", "webui-readiness":
		return app == "open-webui"
	}
	return false
}
func ValidReference(id string) bool { return referencePattern.MatchString(id) }
func ValidAcknowledgement(token string) bool {
	return token == "" || acknowledgementPattern.MatchString(token)
}

// Status keeps the legacy job field while exposing only classified diagnostics.
// One atomic status file prevents stale companion diagnostics describing a new job.
type Status struct {
	Job           string `json:"job"`
	State         string `json:"state"`
	App           string `json:"app,omitempty"`
	Reason        string `json:"reason,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	StatusToken   string `json:"status_token,omitempty"`
}

func ParseStatus(raw string) Status {
	raw = strings.TrimSpace(raw)
	if len(raw) > 100 {
		return Status{Job: "unknown", State: "unknown"}
	}
	if raw == "idle" || raw == "rejected" {
		return Status{Job: raw, State: raw}
	}
	fields := strings.Split(raw, ":")
	if len(fields) != 2 && len(fields) != 4 {
		return Status{Job: "unknown", State: "unknown"}
	}
	state, app := fields[0], fields[1]
	if (app != "ollama" && app != "open-webui") || (state != "installing" && state != "installed" && state != "failed" && state != "uncertain") {
		return Status{Job: "unknown", State: "unknown"}
	}
	s := Status{Job: state + ":" + app, State: state, App: app}
	if len(fields) == 4 {
		if !ValidReason(app, fields[2]) || !ValidReference(fields[3]) || (state != "failed" && state != "uncertain") {
			return Status{Job: "unknown", State: "unknown"}
		}
		s.Reason = fields[2]
		s.CorrelationID = fields[3]
	}
	if state == "uncertain" {
		sum := sha256.Sum256([]byte(raw))
		s.StatusToken = hex.EncodeToString(sum[:])
	}
	return s
}
func ReadStatus(path string) Status {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if os.IsNotExist(err) {
		return ParseStatus("idle")
	}
	if err != nil {
		return ParseStatus("unknown")
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() || info.Size() > 101 {
		return ParseStatus("unknown")
	}
	b, err := io.ReadAll(io.LimitReader(f, 102))
	if err != nil || len(b) > 101 {
		return ParseStatus("unknown")
	}
	return ParseStatus(string(b))
}
