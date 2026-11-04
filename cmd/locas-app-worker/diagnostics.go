package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/p8reit/LocasOS/internal/appmanager"
)

type commandFailure struct {
	reason   string
	exitCode int
}

func (*commandFailure) Error() string { return "fixed installer command failed" }

type installFailure struct {
	reason         string
	exitCode       int
	mayHaveEffects bool
}

func (*installFailure) Error() string { return "application installation did not complete" }
func classifyFailure(err error, fallback string, mayHaveEffects bool) *installFailure {
	failure := &installFailure{reason: fallback, exitCode: -1, mayHaveEffects: mayHaveEffects}
	var command *commandFailure
	if errors.As(err, &command) {
		if command.reason != "installer-failed" {
			failure.reason = command.reason
		}
		failure.exitCode = command.exitCode
	}
	return failure
}

// Only the fixed stage protocol is retained. Subprocess output, arbitrary errors,
// URLs, paths, secrets and bodies are never sent to journald, audit or browser.
type stageWriter struct {
	mu       sync.Mutex
	app      string
	line     []byte
	overflow bool
	last     string
}

func (w *stageWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, b := range data {
		if b == '\n' {
			w.accept()
			w.line = w.line[:0]
			w.overflow = false
			continue
		}
		if len(w.line) >= 160 {
			w.overflow = true
			continue
		}
		w.line = append(w.line, b)
	}
	return len(data), nil
}
func (w *stageWriter) accept() {
	if w.overflow {
		return
	}
	value := strings.TrimSuffix(string(w.line), "\r")
	for _, prefix := range []string{"LOCAS_INSTALL_STAGE=", "LOCAS_INSTALL_ERROR="} {
		if strings.HasPrefix(value, prefix) {
			reason := strings.TrimPrefix(value, prefix)
			if appmanager.ValidReason(w.app, reason) {
				w.last = reason
			}
			return
		}
	}
}
func runCommand(ctx context.Context, path string, args ...string) error {
	app, fallback := "", "installer-failed"
	switch path {
	case "/usr/local/sbin/locas-install-ollama":
		app = "ollama"
		fallback = "ollama-preflight"
	case "/usr/local/sbin/locas-install-open-webui":
		app = "open-webui"
		fallback = "webui-preflight"
	}
	filter := &stageWriter{app: app}
	command := exec.CommandContext(ctx, path, args...)
	command.Stderr = filter
	// nil stdout is discarded; stderr is parsed, never forwarded raw.
	err := command.Run()
	if err == nil {
		return nil
	}
	filter.mu.Lock()
	filter.accept()
	reason := filter.last
	filter.mu.Unlock()
	if reason == "" {
		reason = fallback
	}
	code := -1
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	}
	return &commandFailure{reason: reason, exitCode: code}
}
func (w *worker) failureStatus(state, app, reason, correlation string) error {
	if !appmanager.ValidReason(app, reason) || !appmanager.ValidReference(correlation) {
		return errors.New("invalid classified installer outcome")
	}
	return w.writeStatus(state + ":" + app + ":" + reason + ":" + correlation)
}
func logInstallFailure(app, correlation, outcome string, failure *installFailure) {
	// Every string originates in validated request metadata or fixed classification.
	fmt.Fprintf(os.Stderr, "app_install app=%s stage=%s exit_code=%d outcome=%s correlation_id=%s\n", app, failure.reason, failure.exitCode, outcome, correlation)
}
