package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/p8reit/LocasOS/internal/auditlog"
	"github.com/p8reit/LocasOS/internal/auditop"
)

type memoryRecorder struct {
	events []auditlog.Event
	failAt int
}

func (m *memoryRecorder) Append(e auditlog.Event) error {
	if len(m.events)+1 == m.failAt {
		return errors.New("injected audit failure")
	}
	m.events = append(m.events, e)
	return nil
}
func agentRequest(path string) *http.Request {
	r := httptest.NewRequest("POST", path, strings.NewReader(`{"app":"ollama"}`))
	state := &auditop.State{Attribution: auditop.Attribution{CorrelationID: strings.Repeat("a", 32), Principal: "administrator", SessionID: strings.Repeat("b", 32)}}
	if err := auditop.SetHeaders(r, auditop.With(r.Context(), state)); err != nil {
		panic(err)
	}
	return r.WithContext(auditop.WithPeer(r.Context(), 123))
}
func TestAgentAuditsObservedPeerAndNotUIDHeader(t *testing.T) {
	for _, path := range []string{"/internal/apps/install", "/internal/updates/install"} {
		t.Run(path, func(t *testing.T) {
			recorder := &memoryRecorder{}
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				state := auditop.From(r.Context())
				if state == nil || state.Attribution.PeerUID == nil || *state.Attribution.PeerUID != 123 {
					t.Fatal("observed peer lost")
				}
				w.WriteHeader(202)
			})
			r := agentRequest(path)
			r.Header.Set("X-Locas-Audit-Peer-UID", "0")
			w := httptest.NewRecorder()
			auditMutations(recorder, 123, next).ServeHTTP(w, r)
			if !called || w.Code != 202 || len(recorder.events) != 2 {
				t.Fatalf("status %d events %d called %v", w.Code, len(recorder.events), called)
			}
			for _, e := range recorder.events {
				if e.PeerUID == nil || *e.PeerUID != 123 || e.Source != "core_ipc" || e.SessionID != strings.Repeat("b", 32) {
					t.Fatal("wrong attribution")
				}
			}
		})
	}
}
func TestAgentRejectsInvalidAssertionsAndObservedPeer(t *testing.T) {
	for _, scenario := range []string{"missing header", "duplicate header", "no peer", "wrong peer"} {
		t.Run(scenario, func(t *testing.T) {
			r := agentRequest("/internal/apps/install")
			switch scenario {
			case "missing header":
				r.Header.Del("X-Locas-Audit-Session")
			case "duplicate header":
				r.Header.Add("X-Locas-Audit-Principal", "administrator")
			case "wrong peer":
				r = r.WithContext(auditop.WithPeer(r.Context(), 999))
			case "no peer":
				plain := httptest.NewRequest("POST", r.URL.Path, nil)
				plain.Header = r.Header
				r = plain
			}
			recorder := &memoryRecorder{}
			called := false
			w := httptest.NewRecorder()
			auditMutations(recorder, 123, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(w, r)
			if called || w.Code != 403 || len(recorder.events) != 2 {
				t.Fatalf("status %d events %d called %v", w.Code, len(recorder.events), called)
			}
			for _, e := range recorder.events {
				if e.Principal != "anonymous" || e.SessionID != "" || e.CorrelationID == strings.Repeat("a", 32) {
					t.Fatal("untrusted attribution retained")
				}
			}
		})
	}
}
func TestAgentAuditFailureBlocksActionOrSuccess(t *testing.T) {
	for _, failure := range []int{1, 2} {
		t.Run(string(rune('0'+failure)), func(t *testing.T) {
			recorder := &memoryRecorder{failAt: failure}
			called := false
			w := httptest.NewRecorder()
			auditMutations(recorder, 123, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(202) })).ServeHTTP(w, agentRequest("/internal/apps/install"))
			if w.Code != 503 || called != (failure == 2) {
				t.Fatalf("status %d called %v", w.Code, called)
			}
		})
	}
}
func TestRejectedConnectionHasFreshAnonymousAudit(t *testing.T) {
	recorder := &memoryRecorder{}
	uid := uint32(444)
	recordRejectedPeer(recorder, &uid)
	if len(recorder.events) != 2 {
		t.Fatal("missing connect audit")
	}
	for _, event := range recorder.events {
		if event.Operation != "ipc.connect" || event.Principal != "anonymous" || event.PeerUID == nil || *event.PeerUID != uid {
			t.Fatal("bad connect audit")
		}
	}
}
