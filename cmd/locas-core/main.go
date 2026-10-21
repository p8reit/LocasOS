package main

import (
	"context"
	"encoding/json"
	_ "embed"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/p8reit/LocasOS/internal/agentclient"
	"github.com/p8reit/LocasOS/internal/server"
)

//go:embed index.html
var landingPage []byte

func main() {
	addr := os.Getenv("LOCAS_LISTEN")
	if addr == "" {
		addr = "127.0.0.1:59420"
	}
	socket := os.Getenv("LOCAS_AGENT_SOCKET")
	if socket == "" {
		socket = "/run/locas/agent.sock"
	}

	mux := http.NewServeMux()
	mux.Handle("/", server.NewHandler("locas-core"))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		_, _ = w.Write(landingPage)
	})
	mux.HandleFunc("GET /internal/agent-health", func(w http.ResponseWriter, r *http.Request) {
		// This intentionally exposes only a boolean dependency status, not host inventory or privileges.
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		_, err := agentclient.Check(ctx, socket)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "degraded"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("locas-core listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
