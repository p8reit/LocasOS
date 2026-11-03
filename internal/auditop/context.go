// Package auditop carries server-created operation identity across trusted
// service boundaries. Browser supplied attribution is never used by Core.
package auditop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"

	"github.com/p8reit/LocasOS/internal/auditlog"
)

var idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

type Attribution struct {
	CorrelationID string  `json:"correlation_id"`
	Principal     string  `json:"principal"`
	SessionID     string  `json:"session_id,omitempty"`
	PeerUID       *uint32 `json:"peer_uid,omitempty"`
}

func NewID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}
func (a Attribution) Validate(requirePeer bool) error {
	if !idPattern.MatchString(a.CorrelationID) || a.Principal != "administrator" || !idPattern.MatchString(a.SessionID) || (requirePeer && a.PeerUID == nil) {
		return errors.New("invalid authenticated operation attribution")
	}
	return nil
}
func (a Attribution) SameAssertion(b Attribution) bool {
	return a.CorrelationID == b.CorrelationID && a.Principal == b.Principal && a.SessionID == b.SessionID
}

type State struct {
	Attribution Attribution
	Operation   string
	Source      string
	App         string
	Version     string
	Channel     string
	SHA256      string
	Outcome     string
}
type contextKey struct{}
type peerKey struct{}

func With(ctx context.Context, s *State) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}
func From(ctx context.Context) *State { s, _ := ctx.Value(contextKey{}).(*State); return s }
func WithPeer(ctx context.Context, uid uint32) context.Context {
	return context.WithValue(ctx, peerKey{}, uid)
}
func Peer(ctx context.Context) (uint32, bool) {
	uid, ok := ctx.Value(peerKey{}).(uint32)
	return uid, ok
}
func (s *State) Event(service, stage, outcome string, status int) auditlog.Event {
	return auditlog.Event{CorrelationID: s.Attribution.CorrelationID, Service: service, Operation: s.Operation, Stage: stage, Principal: s.Attribution.Principal, SessionID: s.Attribution.SessionID, Source: s.Source, PeerUID: s.Attribution.PeerUID, Outcome: outcome, HTTPStatus: status, App: s.App, Version: s.Version, Channel: s.Channel, SHA256: s.SHA256}
}
func Record(recorder auditlog.Recorder, event auditlog.Event) error {
	if recorder == nil {
		return errors.New("audit recorder unavailable")
	}
	return recorder.Append(event)
}
func ReadHeaders(r *http.Request) (Attribution, error) {
	a := Attribution{CorrelationID: r.Header.Get("X-Locas-Audit-Correlation"), Principal: r.Header.Get("X-Locas-Audit-Principal"), SessionID: r.Header.Get("X-Locas-Audit-Session")}
	// Duplicate identity headers are ambiguous and rejected, even if first is valid.
	for _, key := range []string{"X-Locas-Audit-Correlation", "X-Locas-Audit-Principal", "X-Locas-Audit-Session"} {
		if len(r.Header.Values(key)) != 1 {
			return Attribution{}, errors.New("missing or duplicate attribution headers")
		}
	}
	return a, a.Validate(false)
}
func SetHeaders(out *http.Request, ctx context.Context) error {
	state := From(ctx)
	if state == nil {
		return errors.New("operation context unavailable")
	}
	a := state.Attribution
	if err := a.Validate(false); err != nil {
		return err
	}
	out.Header.Set("X-Locas-Audit-Correlation", a.CorrelationID)
	out.Header.Set("X-Locas-Audit-Principal", a.Principal)
	out.Header.Set("X-Locas-Audit-Session", a.SessionID)
	return nil
}
