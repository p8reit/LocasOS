package main

import (
	"net/http"
	"strings"

	"github.com/p8reit/LocasOS/internal/admin"
	"github.com/p8reit/LocasOS/internal/auditlog"
	"github.com/p8reit/LocasOS/internal/auditop"
)

func auditOperation(r *http.Request, authenticated bool) string {
	if r.Method == http.MethodPost {
		switch r.URL.Path {
		case "/api/setup":
			return "auth.setup"
		case "/api/login":
			return "auth.login"
		case "/api/logout":
			return "auth.logout"
		case "/api/model/pull":
			return "model.pull"
		case "/api/chat":
			return "model.chat"
		case "/api/apps/install":
			return "app.install"
		case "/api/updates/channel":
			return "update.channel"
		case "/api/updates/install":
			return "update.install"
		}
	}
	if (r.Method == http.MethodGet || r.Method == http.MethodHead) && r.URL.Path == "/api/updates/check" {
		return "update.discover"
	}
	if !authenticated {
		switch r.URL.Path {
		case "/api/telemetry", "/api/runtime", "/api/apps", "/api/updates", "/api/updates/status":
			return "request.denied"
		}
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && strings.HasPrefix(r.URL.Path, "/api/") {
		return "request.denied"
	}
	return ""
}
func coreAuditState(admins *admin.Manager, r *http.Request, operation string) (*auditop.State, error) {
	id, err := auditop.NewID()
	if err != nil {
		return nil, err
	}
	a := auditop.Attribution{CorrelationID: id, Principal: "anonymous"}
	if admins.Authenticated(r) {
		a.Principal = "administrator"
		a.SessionID = admins.AuditSessionID(r)
		if err := a.Validate(false); err != nil {
			return nil, err
		}
	}
	return &auditop.State{Attribution: a, Operation: operation, Source: "browser"}, nil
}
func coreAuditHandler(admins *admin.Manager, journal auditlog.Recorder, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		operation := auditOperation(r, admins.Authenticated(r))
		if operation == "" {
			next.ServeHTTP(w, r)
			return
		}
		state, err := coreAuditState(admins, r, operation)
		if err != nil {
			http.Error(w, "security operation identity unavailable", 503)
			return
		}
		if operation == "model.chat" || operation == "model.pull" {
			state.App = "ollama"
		}
		auditop.Serve(journal, "core", state, w, r, next)
	})
}
func auditHostDenied(admins *admin.Manager, journal auditlog.Recorder, w http.ResponseWriter, r *http.Request) {
	state, err := coreAuditState(admins, r, "request.denied")
	if err != nil {
		http.Error(w, "security operation identity unavailable", 503)
		return
	}
	auditop.Serve(journal, "core", state, w, r, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "unexpected host", 400) }))
}
