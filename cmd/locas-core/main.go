package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/p8reit/LocasOS/internal/agentclient"
	"github.com/p8reit/LocasOS/internal/server"
)

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
