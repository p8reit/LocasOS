package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/p8reit/LocasOS/internal/appmanager"
)

func TestReadOnlyOllamaPrerequisitesFailWithoutInstallerEffects(t *testing.T) {
	for _, apiDown := range []bool{false, true} {
		t.Run(map[bool]string{false: "service-down", true: "api-down"}[apiDown], func(t *testing.T) {
			w, recorder, calls := fixture(t, "open-webui")
			w.ollamaReady = func(context.Context) bool { return false }
			w.execute = func(_ context.Context, path string, args ...string) error {
				*calls = append(*calls, path)
				if path != "/usr/bin/systemctl" {
					t.Fatal("installer executed")
				}
				if !apiDown {
					return &commandFailure{reason: "installer-failed", exitCode: 3}
				}
				return nil
			}
			if err := w.run(context.Background()); err == nil {
				t.Fatal("prerequisite error ignored")
			}
			raw, _ := os.ReadFile(filepath.Join(w.statusDir, "app-install.status"))
			s := appmanager.ParseStatus(string(raw))
			if s.State != "failed" || s.Reason != "ollama-required" || len(s.CorrelationID) != 32 || recorder.events[1].Outcome != "failed" || len(*calls) != 1 {
				t.Fatalf("%+v %+v", s, recorder.events)
			}
		})
	}
}
func TestPartialWebUIEffectsRemainUncertainWithClassifiedReason(t *testing.T) {
	w, recorder, _ := fixture(t, "open-webui")
	count := 0
	w.execute = func(context.Context, string, ...string) error {
		count++
		if count == 2 {
			return &commandFailure{reason: "webui-readiness", exitCode: 23}
		}
		return nil
	}
	if w.run(context.Background()) == nil {
		t.Fatal("failure ignored")
	}
	raw, _ := os.ReadFile(filepath.Join(w.statusDir, "app-install.status"))
	s := appmanager.ParseStatus(string(raw))
	if s.State != "uncertain" || s.Reason != "webui-readiness" || recorder.events[1].Outcome != "uncertain" {
		t.Fatalf("%+v", s)
	}
}
func TestSystemCommandPreservesCallerClassification(t *testing.T) {
	f := classifyFailure(&commandFailure{reason: "installer-failed", exitCode: 3}, "ollama-required", false)
	if f.reason != "ollama-required" || f.exitCode != 3 || f.mayHaveEffects {
		t.Fatal(f)
	}
}
func TestStageParserBoundsAndSuppressesRawOutput(t *testing.T) {
	w := &stageWriter{app: "open-webui"}
	for _, chunk := range []string{"SECRET raw credential\nLOCAS_INSTALL_STA", "GE=webui-docker-install\n", "LOCAS_INSTALL_ERROR=SECRET\n", strings.Repeat("x", 1000) + "LOCAS_INSTALL_ERROR=webui-pull\n", "LOCAS_INSTALL_STAGE=ollama-extract\n"} {
		_, _ = w.Write([]byte(chunk))
	}
	if w.last != "webui-docker-install" || len(w.line) > 160 {
		t.Fatal("invalid/unbounded diagnostic retention")
	}
}
func TestCommandErrorsNeverContainProcessOutputOrArguments(t *testing.T) {
	// Executes only this fixed test child, not a host installer or network command.
	err := runCommand(context.Background(), "/bin/sh", "-c", "printf 'SECRET ARGUMENT CREDENTIAL\\n' >&2; exit 7")
	var failure *commandFailure
	if !errors.As(err, &failure) || failure.exitCode != 7 || failure.reason != "installer-failed" || strings.Contains(err.Error(), "SECRET") {
		t.Fatal("raw diagnostic escaped")
	}
}
func TestPostActionAuditFailureKeepsUncertainReference(t *testing.T) {
	w, recorder, _ := fixture(t, "ollama")
	recorder.failAt = 2
	if w.run(context.Background()) == nil {
		t.Fatal("audit failure ignored")
	}
	raw, _ := os.ReadFile(filepath.Join(w.statusDir, "app-install.status"))
	s := appmanager.ParseStatus(string(raw))
	if s.State != "uncertain" || s.Reason != "audit-outcome" || s.CorrelationID != strings.Repeat("a", 32) {
		t.Fatalf("%+v", s)
	}
}
