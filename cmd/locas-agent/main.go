package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/p8reit/LocasOS/internal/agentapi"
	"github.com/p8reit/LocasOS/internal/appmanager"
	"github.com/p8reit/LocasOS/internal/updatequeue"
	"github.com/p8reit/LocasOS/internal/gpu"
	"github.com/p8reit/LocasOS/internal/hardware"
	"github.com/p8reit/LocasOS/internal/peercred"
	"github.com/p8reit/LocasOS/internal/server"
)

func main() {
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
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("locas-agent listening on unix socket %s (allowed uid: %d)", socket, uid)
	log.Fatal(srv.Serve(&peercred.Listener{UnixListener: listener, ExpectedUID: uint32(uid)}))
}
