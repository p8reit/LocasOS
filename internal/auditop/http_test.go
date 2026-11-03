package auditop

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/p8reit/LocasOS/internal/auditlog"
)

type recording struct {
	events []auditlog.Event
	fail   int
}

func (s *recording) Append(e auditlog.Event) error {
	s.events = append(s.events, e)
	if len(s.events) == s.fail {
		return errors.New("PRIVATE recorder detail")
	}
	return nil
}
func testState() *State {
	return &State{Attribution: Attribution{CorrelationID: strings.Repeat("a", 32), Principal: "administrator", SessionID: strings.Repeat("b", 32)}, Operation: "model.pull", Source: "browser"}
}

func TestServeDurabilityAndResponses(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		fail, status           int
		overflow, panicHandler bool
		wantOutcome            string
	}{
		{name: "intent failure", fail: 1, status: 503},
		{name: "result failure", fail: 2, status: 503, wantOutcome: "succeeded"},
		{name: "success", status: 201, wantOutcome: "succeeded"},
		{name: "accepted", status: 202, wantOutcome: "accepted"},
		{name: "denied", status: 403, wantOutcome: "denied"},
		{name: "failure", status: 500, wantOutcome: "failed"},
		{name: "overflow", status: 503, overflow: true, wantOutcome: "uncertain"},
		{name: "panic", status: 503, panicHandler: true, wantOutcome: "uncertain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recording{fail: tc.fail}
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/private?token=PRIVATE", strings.NewReader("PRIVATE request"))
			r.Header.Set("Cookie", "PRIVATE cookie")
			r.Header.Set("X-Locas-Correlation-ID", "spoofed")
			s := testState()
			called := false
			Serve(rec, "core", s, w, r, http.HandlerFunc(func(out http.ResponseWriter, in *http.Request) {
				called = true
				if From(in.Context()) != s {
					t.Error("missing operation context")
				}
				out.Header().Set("Set-Cookie", "PRIVATE credential")
				out.Header().Set("X-Normal", "preserved")
				out.Header().Set("X-Locas-Correlation-ID", "spoofed")
				out.Header().Set("Cache-Control", "public")
				code := tc.status
				if code == 503 {
					code = 201
				}
				out.WriteHeader(code)
				out.Write([]byte("PRIVATE response"))
				if tc.panicHandler {
					panic("PRIVATE panic")
				}
				if tc.overflow {
					out.Write([]byte(strings.Repeat("x", maxResponse)))
				}
			}))
			if called != (tc.fail != 1) {
				t.Fatalf("handler called %v", called)
			}
			if w.Code != tc.status {
				t.Fatalf("status %d", w.Code)
			}
			if w.Header().Get("X-Locas-Correlation-ID") != s.Attribution.CorrelationID || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("authoritative headers lost")
			}
			failed := tc.fail != 0 || tc.overflow || tc.panicHandler
			if failed {
				if strings.Contains(w.Body.String(), "PRIVATE") || len(w.Header().Values("Set-Cookie")) != 0 || w.Header().Get("X-Normal") != "" {
					t.Fatalf("private handler output leaked: %v %s", w.Header(), w.Body.String())
				}
			} else if w.Body.String() != "PRIVATE response" || w.Header().Get("Set-Cookie") != "PRIVATE credential" || w.Header().Get("X-Normal") != "preserved" {
				t.Fatal("normal output lost")
			}
			if len(rec.events) != 1 && tc.fail == 1 {
				t.Fatal("result recorded after intent failure")
			}
			if tc.fail != 1 {
				if len(rec.events) != 2 || rec.events[1].Outcome != tc.wantOutcome || rec.events[1].Stage != "result" {
					t.Fatalf("events %+v", rec.events)
				}
			}
			encoded, _ := json.Marshal(rec.events)
			if strings.Contains(string(encoded), "PRIVATE") {
				t.Fatal("private request, response, or panic recorded")
			}
		})
	}
}

func TestServeNilRecorderAndEmptySuccess(t *testing.T) {
	w := httptest.NewRecorder()
	called := false
	Serve(nil, "core", testState(), w, httptest.NewRequest("POST", "/", nil), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	if called || w.Code != 503 {
		t.Fatal("nil recorder did not block handler")
	}
	rec := &recording{}
	w = httptest.NewRecorder()
	Serve(rec, "core", testState(), w, httptest.NewRequest("POST", "/", nil), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	if w.Code != 200 || rec.events[1].HTTPStatus != 200 {
		t.Fatal("empty success not audited")
	}
}

func TestPanicWithFailedOutcomeAudit(t *testing.T) {
	rec := &recording{fail: 2}
	s := testState()
	s.Outcome = "succeeded"
	w := httptest.NewRecorder()
	Serve(rec, "core", s, w, httptest.NewRequest("POST", "/", nil), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("PRIVATE") }))
	if w.Code != 503 || rec.events[1].Outcome != "uncertain" || strings.Contains(w.Body.String(), "PRIVATE") {
		t.Fatal("panic failed closed incorrectly")
	}
}

func TestContextAndHeaderValidation(t *testing.T) {
	ctx := context.Background()
	if From(ctx) != nil {
		t.Fatal("unexpected state")
	}
	if _, ok := Peer(ctx); ok {
		t.Fatal("unexpected peer")
	}
	ctx = WithPeer(ctx, 123)
	if uid, ok := Peer(ctx); !ok || uid != 123 {
		t.Fatal("peer lost")
	}
	s := testState()
	ctx = With(ctx, s)
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("X-Locas-Audit-Principal", "spoofed")
	if err := SetHeaders(r, ctx); err != nil {
		t.Fatal(err)
	}
	a, err := ReadHeaders(r)
	if err != nil || !a.SameAssertion(s.Attribution) || a.PeerUID != nil {
		t.Fatalf("%+v %v", a, err)
	}
	for _, key := range []string{"X-Locas-Audit-Correlation", "X-Locas-Audit-Principal", "X-Locas-Audit-Session"} {
		copy := r.Clone(ctx)
		copy.Header.Add(key, r.Header.Get(key))
		if _, err := ReadHeaders(copy); err == nil {
			t.Fatalf("accepted duplicate %s", key)
		}
		copy = r.Clone(ctx)
		copy.Header.Del(key)
		if _, err := ReadHeaders(copy); err == nil {
			t.Fatalf("accepted missing %s", key)
		}
	}
	if err := s.Attribution.Validate(true); err == nil {
		t.Fatal("missing observed peer accepted")
	}
	if err := SetHeaders(r, context.Background()); err == nil {
		t.Fatal("missing state accepted")
	}
	s.Attribution.SessionID = "PRIVATE credential"
	if err := SetHeaders(r, ctx); err == nil {
		t.Fatal("invalid session accepted")
	}
}

func TestNewIDAndIndependentAssertion(t *testing.T) {
	a, err := NewID()
	if err != nil || !idPattern.MatchString(a) {
		t.Fatal("invalid random ID")
	}
	b, err := NewID()
	if err != nil || a == b {
		t.Fatal("repeated random ID")
	}
	one := testState().Attribution
	two := one
	uid := uint32(123)
	two.PeerUID = &uid
	if !one.SameAssertion(two) {
		t.Fatal("receiver-observed peer changed asserted identity")
	}
	two.SessionID = a
	if one.SameAssertion(two) {
		t.Fatal("different session matched")
	}
}
