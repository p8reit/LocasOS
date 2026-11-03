package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/p8reit/LocasOS/internal/auditlog"
	"github.com/p8reit/LocasOS/internal/auditop"
	"github.com/p8reit/LocasOS/internal/updateapproval"
)

type faultAudit struct {
	journal auditlog.Recorder
	calls   int
	failAt  int
	events  []auditlog.Event
}

func (f *faultAudit) Append(e auditlog.Event) error {
	f.calls++
	if f.calls == f.failAt {
		return errors.New("injected persistence failure")
	}
	if err := f.journal.Append(e); err != nil {
		return err
	}
	f.events = append(f.events, e)
	return nil
}
func auditFixture(t *testing.T) (request, []byte, *faultAudit, string) {
	t.Helper()
	peer := uint32(123)
	req := request{Selection: updateapproval.Selection{Version: "v1.2.3", Channel: "stable", SHA256: strings.Repeat("a", 64), Token: strings.Repeat("b", 64)}, ExpiresAt: time.Now().Add(time.Minute), Audit: auditop.Attribution{CorrelationID: strings.Repeat("c", 32), Principal: "administrator", SessionID: strings.Repeat("d", 32), PeerUID: &peer}}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "audit")
	journal, err := auditlog.Open(dir, "updater")
	if err != nil {
		t.Fatal(err)
	}
	return req, data, &faultAudit{journal: journal}, dir
}
func TestInvalidUpdateJobGetsDurableDenialWithoutConsumption(t *testing.T) {
	for _, scenario := range []string{"overflow version", "oversize version", "unknown field", "oversize body", "invalid attribution", "expired", "trailing JSON"} {
		t.Run(scenario, func(t *testing.T) {
			req, data, rec, dir := auditFixture(t)
			switch scenario {
			case "overflow version":
				req.Version = "v999999999999999999999999.0.0"
			case "oversize version":
				req.Version = "v" + strings.Repeat("0", 64) + "1.0.0"
			case "invalid attribution":
				req.Audit.SessionID = "browser-password-must-not-log"
			case "expired":
				req.ExpiresAt = time.Now().Add(-time.Minute)
			}
			data, _ = json.Marshal(req)
			switch scenario {
			case "unknown field":
				data = append(data[:len(data)-1], []byte(`,"password":"secret-never-audit"}`)...)
			case "oversize body":
				data = append(data, []byte(strings.Repeat(" ", 1025))...)
			case "trailing JSON":
				data = append(data, []byte(` {"password":"secret-never-audit"}`)...)
			}
			consumed := false
			_, _, rejected, err := beginAndConsumeUpdateAudit(data, rec, time.Now(), func() error { consumed = true; return nil })
			if err == nil || !rejected || consumed {
				t.Fatalf("err=%v rejected=%v consumed=%v", err, rejected, consumed)
			}
			report, err := auditlog.Verify(dir)
			if err != nil || report.Entries != 2 || len(report.Pending) != 0 || rec.events[1].Outcome != "denied" {
				t.Fatalf("report=%+v err=%v events=%+v", report, err, rec.events)
			}
			journal, _ := os.ReadFile(filepath.Join(dir, "events.jsonl"))
			for _, secret := range []string{"secret-never-audit", "browser-password", req.Token} {
				if strings.Contains(string(journal), secret) {
					t.Fatal("secret persisted")
				}
			}
		})
	}
}
func TestUpdateIntentFailureRetainsQueueAndPreventsEffects(t *testing.T) {
	_, data, rec, _ := auditFixture(t)
	rec.failAt = 1
	queue := filepath.Join(t.TempDir(), "request")
	if err := os.WriteFile(queue, data, 0600); err != nil {
		t.Fatal(err)
	}
	_, _, rejected, err := beginAndConsumeUpdateAudit(data, rec, time.Now(), func() error { return os.Remove(queue) })
	if err == nil || rejected {
		t.Fatal("intent failure not distinguished from audited rejection")
	}
	if _, err := os.Stat(queue); err != nil {
		t.Fatal("queue consumed before durable intent")
	}
	if len(rec.events) != 0 {
		t.Fatal("failed intent recorded as durable")
	}
}
func TestUpdateCheckpointFailuresNeverPublishInstalled(t *testing.T) {
	for _, scenario := range []string{"verified", "apply audit", "terminal", "installer", "record persistence", "success"} {
		t.Run(scenario, func(t *testing.T) {
			req, data, rec, dir := auditFixture(t)
			_, state, _, err := beginAndConsumeUpdateAudit(data, rec, time.Now(), func() error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "verified":
				rec.failAt = 2
			case "apply audit":
				rec.failAt = 3
			case "terminal":
				rec.failAt = 4
			}
			actions, persisted := 0, 0
			var statuses []status
			err = applyVerifiedUpdate(rec, state, req, func() error {
				actions++
				if scenario == "installer" {
					return errors.New("secret external error")
				}
				return nil
			}, func() error {
				persisted++
				if scenario == "record persistence" {
					return errors.New("secret external error")
				}
				return nil
			}, func(s status) { statuses = append(statuses, s) })
			if (err == nil) != (scenario == "success") {
				t.Fatalf("unexpected error %v", err)
			}
			if scenario == "verified" || scenario == "apply audit" {
				if actions != 0 || persisted != 0 {
					t.Fatal("action ran before durable checkpoint")
				}
			} else if actions != 1 {
				t.Fatal("installer count")
			}
			if scenario == "installer" && persisted != 0 {
				t.Fatal("persisted version after failed installer")
			}
			for _, s := range statuses {
				if s.State == "installed" && scenario != "success" {
					t.Fatal("normal success after failure")
				}
				if strings.Contains(s.Message, "secret") {
					t.Fatal("raw error published")
				}
			}
			report, verifyErr := auditlog.Verify(dir)
			if verifyErr != nil {
				t.Fatal(verifyErr)
			}
			if scenario == "terminal" {
				if len(report.Pending) != 1 {
					t.Fatal("missing uncertain pending operation")
				}
			} else if len(report.Pending) != 0 {
				t.Fatal("terminal outcome absent")
			}
			if scenario == "installer" || scenario == "record persistence" {
				if len(report.Uncertain) != 1 {
					t.Fatal("partial mutation reported as clean failure")
				}
			}
			if scenario == "success" {
				if statuses[len(statuses)-1].State != "installed" || persisted != 1 {
					t.Fatal("missing healthy persisted success")
				}
			}
		})
	}
}

func TestRejectedUpdateRetainedWhenDenialAuditFails(t *testing.T) {
	req, _, rec, dir := auditFixture(t)
	req.Version = "v999999999999999999999999.0.0"
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	rec.failAt = 2
	consumed := false
	_, _, rejected, err := beginAndConsumeUpdateAudit(data, rec, time.Now(), func() error { consumed = true; return nil })
	if err == nil || rejected || consumed {
		t.Fatal("request eligible for removal without durable rejection")
	}
	report, err := auditlog.Verify(dir)
	if err != nil || len(report.Pending) != 1 {
		t.Fatalf("missing unmatched intent: %+v %v", report, err)
	}
}

func TestQueueConsumptionFailureGetsTerminalAudit(t *testing.T) {
	_, data, rec, dir := auditFixture(t)
	_, _, rejected, err := beginAndConsumeUpdateAudit(data, rec, time.Now(), func() error { return errors.New("secret filesystem path") })
	if err == nil || rejected || strings.Contains(err.Error(), "secret") {
		t.Fatal("invalid consumption failure result")
	}
	report, err := auditlog.Verify(dir)
	if err != nil || len(report.Pending) != 0 || rec.events[len(rec.events)-1].Outcome != "failed" {
		t.Fatalf("missing terminal failure: %+v %v", report, err)
	}
}
