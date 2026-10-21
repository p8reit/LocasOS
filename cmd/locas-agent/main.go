package main

import (
	"github.com/p8reit/LocasOS/internal/server"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	socket := os.Getenv("LOCAS_AGENT_SOCKET")
	if socket == "" {
		socket = "/run/locas/agent.sock"
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
	listener, err := net.Listen("unix", socket)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(socket, 0660); err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{Handler: server.NewHandler("locas-agent"), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("locas-agent listening on unix socket %s", socket)
	log.Fatal(srv.Serve(listener))
}
