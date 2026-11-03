package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/p8reit/LocasOS/internal/agentapi"
	"github.com/p8reit/LocasOS/internal/appmanager"
	"github.com/p8reit/LocasOS/internal/auditlog"
	"github.com/p8reit/LocasOS/internal/auditop"
	"github.com/p8reit/LocasOS/internal/gpu"
	"github.com/p8reit/LocasOS/internal/hardware"
	"github.com/p8reit/LocasOS/internal/peercred"
	"github.com/p8reit/LocasOS/internal/server"
	"github.com/p8reit/LocasOS/internal/updatequeue"
)

func main() {
	journal, err := auditlog.Open("/var/lib/locas-agent-audit", "agent")
	if err != nil {
		log.Fatal("agent audit unavailable")
	}

	socket := os.Getenv("LOCAS_AGENT_SOCKET")
	if socket == "" {
		socket = "/run/locas/agent.sock"
	}
	uidText := os.Getenv("LOCAS_CORE_UID")
	if uidText == "" {
		log.Fatal("LOCAS_CORE_UID is required to restrict agent socket clients")
	}
	uid, err := strconv.ParseUint(uidText, 10, 32)
	if err != nil {
		log.Fatalf("invalid LOCAS_CORE_UID: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(socket), 0750); err != nil {
		log.Fatal(err)
	}
	if info, err := os.Lstat(socket); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			log.Fatalf("refusing to replace non-socket %s", socket)
		}
		if err := os.Remove(socket); err != nil {
			log.Fatal(err)
		}
	} else if !os.IsNotExist(err) {
		log.Fatal(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(socket, 0660); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/healthz", server.NewHandler("locas-agent"))
	mux.Handle("/internal/apps/", http.StripPrefix("/internal/apps", appmanager.New("/var/lib/locas").Handler()))
	mux.Handle("/internal/updates/", http.StripPrefix("/internal/updates", updatequeue.New("/var/lib/locas").Handler()))
	mux.Handle("/internal/telemetry", agentapi.NewTelemetryHandler(hardware.Collect, gpu.Probe))
	srv := &http.Server{Handler: auditMutations(journal, uint32(uid), mux), ReadHeaderTimeout: 5 * time.Second,
		ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
			if unix, ok := conn.(*net.UnixConn); ok {
				if peer, err := peercred.UID(unix); err == nil {
					return auditop.WithPeer(ctx, peer)
				}
			}
			return ctx
		}}
	log.Printf("locas-agent listening on unix socket %s (allowed uid: %d)", socket, uid)
	log.Fatal(srv.Serve(&peercred.Listener{UnixListener: listener, ExpectedUID: uint32(uid), OnReject: func(peer *uint32) { recordRejectedPeer(journal, peer) }}))
}

// Peer identity comes only from SO_PEERCRED, never an HTTP assertion.
func auditMutations(recorder auditlog.Recorder, expected uint32, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		operation := ""
		switch r.URL.Path {
		case "/internal/apps/install":
			operation = "app.install"
		case "/internal/updates/install":
			operation = "update.install"
		}
		if operation == "" {
			next.ServeHTTP(w, r)
			return
		}
		assertion, err := auditop.ReadHeaders(r)
		peer, observed := auditop.Peer(r.Context())
		state := &auditop.State{Attribution: assertion, Operation: operation, Source: "core_ipc"}
		allowed := err == nil && observed && peer == expected
		if observed {
			state.Attribution.PeerUID = &peer
		}
		handler := next
		if !allowed {
			id, e := auditop.NewID()
			if e != nil {
				http.Error(w, "operation identity unavailable", 503)
				return
			}
			state.Attribution = auditop.Attribution{CorrelationID: id, Principal: "anonymous"}
			if observed {
				state.Attribution.PeerUID = &peer
			}
			handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "invalid operation attribution", http.StatusForbidden)
			})
		}
		auditop.Serve(recorder, "agent", state, w, r, handler)
	})
}
func recordRejectedPeer(recorder auditlog.Recorder, peer *uint32) {
	id, err := auditop.NewID()
	if err != nil {
		return
	}
	state := &auditop.State{Attribution: auditop.Attribution{CorrelationID: id, Principal: "anonymous", PeerUID: peer}, Operation: "ipc.connect", Source: "core_ipc"}
	if auditop.Record(recorder, state.Event("agent", "intent", "pending", 0)) != nil {
		return
	}
	_ = auditop.Record(recorder, state.Event("agent", "result", "denied", 0))
}
