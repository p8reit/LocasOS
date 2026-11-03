// locas-app-worker consumes only attributed, allowlisted agent queue requests.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/p8reit/LocasOS/internal/appmanager"
	"github.com/p8reit/LocasOS/internal/auditlog"
	"github.com/p8reit/LocasOS/internal/auditop"
	"github.com/p8reit/LocasOS/internal/updateapproval"
)

type executor func(context.Context, string, ...string) error
type worker struct {
	requestPath string
	statusDir   string
	queueUID    uint32
	recorder    auditlog.Recorder
	execute     executor
	installed   func() bool
}

func main() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "app worker requires root")
		os.Exit(1)
	}
	journal, err := auditlog.Open("/var/lib/locas-audit/app-worker", "app-worker")
	if err != nil {
		fmt.Fprintln(os.Stderr, "app worker audit unavailable")
		os.Exit(1)
	}
	identity, err := user.Lookup("locas-agent")
	if err != nil {
		fmt.Fprintln(os.Stderr, "agent identity unavailable")
		os.Exit(1)
	}
	uid, err := strconv.ParseUint(identity.Uid, 10, 32)
	if err != nil {
		fmt.Fprintln(os.Stderr, "agent identity invalid")
		os.Exit(1)
	}
	w := worker{requestPath: "/var/lib/locas/app-install.request", statusDir: "/var/lib/locas-apps", queueUID: uint32(uid), recorder: journal,
		execute: func(ctx context.Context, path string, args ...string) error {
			return exec.CommandContext(ctx, path, args...).Run()
		},
		installed: func() bool {
			st, err := os.Stat("/usr/local/bin/ollama")
			return err == nil && st.Mode().IsRegular() && st.Mode().Perm()&0111 != 0
		}}
	ctx, cancel := context.WithTimeout(context.Background(), 44*time.Minute)
	defer cancel()
	if w.run(ctx) != nil {
		fmt.Fprintln(os.Stderr, "app worker failed; inspect private audit and status")
		os.Exit(1)
	}
}

// readRequest opens without following links or blocking on FIFOs, then checks
// the descriptor itself before decoding. Plaintext legacy requests fail closed.
func readRequest(path string, uid uint32) (appmanager.Request, error) {
	var req appmanager.Request
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return req, err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	var st syscall.Stat_t
	if syscall.Fstat(fd, &st) != nil || st.Mode&syscall.S_IFMT != syscall.S_IFREG || st.Uid != uid || st.Mode&0027 != 0 || st.Size <= 0 || st.Size > 4096 {
		return req, errors.New("unsafe app request")
	}
	return decodeRequest(f)
}

// A request may grow after fstat. Bound the bytes actually read, including
// whitespace, before decoding so a valid JSON prefix cannot bypass the cap.
func decodeRequest(reader io.Reader) (appmanager.Request, error) {
	var req appmanager.Request
	data, err := io.ReadAll(io.LimitReader(reader, 4097))
	if err != nil || len(data) == 0 || len(data) > 4096 {
		return req, errors.New("oversized or unreadable app request")
	}
	if err := updateapproval.Decode(bytes.NewReader(data), &req); err != nil {
		return req, errors.New("invalid app request")
	}
	if err := req.Validate(); err != nil {
		return req, errors.New("invalid app attribution or application")
	}
	return req, nil
}

func (w *worker) run(ctx context.Context) error {
	if err := os.MkdirAll(w.statusDir, 0755); err != nil {
		return err
	}
	fd, err := syscall.Open(filepath.Join(w.statusDir, "worker.lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	lock := os.NewFile(uintptr(fd), "app worker lock")
	defer lock.Close()
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("app worker already running")
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	req, readErr := readRequest(w.requestPath, w.queueUID)
	if errors.Is(readErr, os.ErrNotExist) {
		return nil
	}
	state := &auditop.State{Attribution: req.Audit, Operation: "app.install", Source: "agent_queue", App: req.App}
	if readErr != nil {
		id, err := auditop.NewID()
		if err != nil {
			return err
		}
		state.Attribution = auditop.Attribution{CorrelationID: id, Principal: "anonymous"}
		state.App = ""
	}
	if err := auditop.Record(w.recorder, state.Event("app-worker", "intent", "pending", 0)); err != nil {
		return errors.New("audit intent unavailable")
	}
	finish := func(outcome string) error {
		return auditop.Record(w.recorder, state.Event("app-worker", "result", outcome, 0))
	}
	if readErr != nil {
		// No installer executes for malformed/legacy jobs; retain an audited denial.
		if err := finish("denied"); err != nil {
			return errors.New("audit denial unavailable")
		}
		_ = os.Remove(w.requestPath)
		_ = w.writeStatus("rejected")
		return errors.New("app request rejected")
	}
	if err := os.Remove(w.requestPath); err != nil {
		_ = finish("failed")
		return errors.New("app request consume failed")
	}
	if err := syncDir(filepath.Dir(w.requestPath)); err != nil {
		_ = finish("uncertain")
		return errors.New("app request consume durability unavailable")
	}
	if err := w.writeStatus("installing:" + req.App); err != nil {
		_ = finish("failed")
		return err
	}
	executeErr := w.install(ctx, req.App)
	if executeErr != nil {
		// An executor error can follow partial package, service or data changes.
		// We cannot prove a rollback; record uncertainty rather than no effect.
		if err := finish("uncertain"); err != nil {
			_ = w.writeStatus("uncertain:" + req.App)
			return errors.New("audit failure after installer failure")
		}
		_ = w.writeStatus("uncertain:" + req.App)
		return errors.New("application installation outcome uncertain")
	}
	// Never expose installed status until the outcome is durable.
	if err := finish("succeeded"); err != nil {
		_ = w.writeStatus("uncertain:" + req.App)
		return errors.New("audit outcome unavailable after installer")
	}
	return w.writeStatus("installed:" + req.App)
}
func (w *worker) install(ctx context.Context, app string) error {
	switch app {
	case "ollama":
		if w.installed != nil && w.installed() {
			if err := w.execute(ctx, "/usr/bin/systemctl", "enable", "--now", "locas-ollama.service"); err != nil {
				return err
			}
		} else {
			if err := w.execute(ctx, "/usr/local/sbin/locas-install-ollama"); err != nil {
				return err
			}
		}
		return w.execute(ctx, "/usr/bin/systemctl", "is-active", "--quiet", "locas-ollama.service")
	case "open-webui":
		if err := w.execute(ctx, "/usr/bin/systemctl", "is-active", "--quiet", "locas-ollama.service"); err != nil {
			return err
		}
		return w.execute(ctx, "/usr/local/sbin/locas-install-open-webui")
	}
	return errors.New("unsupported application")
}
func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func (w *worker) writeStatus(status string) error {
	f, err := os.CreateTemp(w.statusDir, "status-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0644); err == nil {
		_, err = f.WriteString(status + "\n")
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, filepath.Join(w.statusDir, "app-install.status")); err != nil {
		return err
	}
	return syncDir(w.statusDir)
}
