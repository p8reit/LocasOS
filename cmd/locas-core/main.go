package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/p8reit/LocasOS/internal/admin"
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
	stateDir := os.Getenv("LOCAS_ADMIN_STATE_DIR")
	if stateDir == "" {
		stateDir = "/var/lib/locas-core"
	}
	admins, err := admin.New(stateDir)
	if err != nil {
		log.Fatalf("administrator state unavailable: %v", err)
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
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		sendJSON(w, http.StatusOK, map[string]bool{
			"configured":    admins.Configured(),
			"authenticated": admins.Authenticated(r),
		})
	})
	mux.HandleFunc("POST /api/setup", func(w http.ResponseWriter, r *http.Request) {
		if !admin.SafeMutation(r) {
			http.Error(w, "invalid request", http.StatusForbidden)
			return
		}
		password, err := admin.DecodePassword(w, r)
		if err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := admins.Setup(password); err != nil {
			http.Error(w, "setup rejected: administrator exists or password is invalid", http.StatusConflict)
			return
		}
		sendJSON(w, http.StatusCreated, map[string]string{"status": "configured"})
	})
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		if !admin.SafeMutation(r) {
			http.Error(w, "invalid request", http.StatusForbidden)
			return
		}
		password, err := admin.DecodePassword(w, r)
		if err != nil || len(password) > 256 {
			http.Error(w, "invalid login request", http.StatusBadRequest)
			return
		}
		key, ok := admins.Login(password, r.RemoteAddr)
		if !ok {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		admin.SetCookie(w, key, r.TLS != nil)
		sendJSON(w, http.StatusOK, map[string]string{"status": "authenticated"})
	})
	mux.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		if !admin.SafeMutation(r) {
			http.Error(w, "invalid request", http.StatusForbidden)
			return
		}
		admins.Logout(r)
		admin.ClearCookie(w)
		sendJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	})
	mux.HandleFunc("GET /api/telemetry", func(w http.ResponseWriter, r *http.Request) {
		if !admins.Authenticated(r) {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", socket)
			},
		}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://agent/internal/telemetry", nil)
		if err != nil {
			http.Error(w, "agent request failed", http.StatusServiceUnavailable)
			return
		}
		resp, err := client.Do(request)
		if err != nil {
			http.Error(w, "agent unavailable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			http.Error(w, "telemetry unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.Copy(w, io.LimitReader(resp.Body, 65536))
	})
	mux.HandleFunc("GET /api/runtime", func(w http.ResponseWriter, r *http.Request) {
		if !admins.Authenticated(r) {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		// The target is fixed at loopback inside LocasOS. No user-defined URLs,
		// arbitrary proxying, model downloads or runtime mutations.
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		client := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:11434/api/tags", nil)
		resp, err := client.Do(req)
		if err != nil {
			sendJSON(w, http.StatusOK, map[string]any{"ollama": "unavailable", "models": []string{}, "scope": "LocasOS"})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			sendJSON(w, http.StatusOK, map[string]any{"ollama": "unavailable", "models": []string{}, "scope": "LocasOS"})
			return
		}
		var data struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1048576)).Decode(&data); err != nil {
			sendJSON(w, http.StatusOK, map[string]any{"ollama": "unavailable", "models": []string{}, "scope": "LocasOS"})
			return
		}
		models := make([]string, 0, len(data.Models))
		for _, model := range data.Models {
			if len(model.Name) <= 128 {
				models = append(models, model.Name)
			}
			if len(models) == 100 {
				break
			}
		}
		sendJSON(w, http.StatusOK, map[string]any{"ollama": "available", "models": models, "scope": "LocasOS"})
	})
	// Phase 6C: authenticated local-only operations. Never accept URLs or shell commands.
	mux.HandleFunc("POST /api/model/pull", func(w http.ResponseWriter, r *http.Request) {
		if !admins.Authenticated(r) {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if !admin.SafeMutation(r) {
			http.Error(w, "invalid request", http.StatusForbidden)
			return
		}
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&payload); err != nil {
			http.Error(w, "invalid model request", http.StatusBadRequest)
			return
		}
		allowed := map[string]bool{"qwen2.5:3b": true, "qwen2.5-coder:7b": true, "llama3.2:3b": true}
		if !allowed[payload.Model] {
			http.Error(w, "model not approved for preview", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
		defer cancel()
		client := &http.Client{Timeout: 15 * time.Minute}
		body, _ := json.Marshal(map[string]any{"model": payload.Model, "stream": false})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:11434/api/pull", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "local runtime unavailable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			http.Error(w, "model pull failed", http.StatusBadGateway)
			return
		}
		var outcome map[string]any
		if err := json.NewDecoder(io.LimitReader(resp.Body, 16384)).Decode(&outcome); err != nil {
			http.Error(w, "invalid runtime response", http.StatusBadGateway)
			return
		}
		sendJSON(w, http.StatusOK, map[string]any{"status": "download_complete", "model": payload.Model})
	})
	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		if !admins.Authenticated(r) {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if !admin.SafeMutation(r) {
			http.Error(w, "invalid request", http.StatusForbidden)
			return
		}
		var payload struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil || len(payload.Prompt) == 0 || len(payload.Prompt) > 6000 {
			http.Error(w, "invalid chat request", http.StatusBadRequest)
			return
		}
		if len(payload.Model) == 0 || len(payload.Model) > 128 || strings.ContainsAny(payload.Model, " /") {
			http.Error(w, "invalid model", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		data, _ := json.Marshal(map[string]any{"model": payload.Model, "stream": false, "messages": []map[string]string{{"role": "user", "content": payload.Prompt}}, "options": map[string]int{"num_predict": 256}})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:11434/api/chat", strings.NewReader(string(data)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
		if err != nil {
			http.Error(w, "local inference unavailable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			http.Error(w, "local inference failed or model missing", http.StatusBadGateway)
			return
		}
		var answer struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1048576)).Decode(&answer); err != nil {
			http.Error(w, "invalid inference result", http.StatusBadGateway)
			return
		}
		sendJSON(w, http.StatusOK, map[string]string{"reply": answer.Message.Content})
	})
	// Authenticated application-manager proxy: only fixed routes on the peer-checked Unix agent socket.
	mux.HandleFunc("GET /api/apps", func(w http.ResponseWriter, r *http.Request) {
		if !admins.Authenticated(r) { http.Error(w, "authentication required", http.StatusUnauthorized); return }
		appProxy(w, r, socket, http.MethodGet, "/internal/apps/status", nil)
	})
	mux.HandleFunc("POST /api/apps/install", func(w http.ResponseWriter, r *http.Request) {
		if !admins.Authenticated(r) { http.Error(w, "authentication required", http.StatusUnauthorized); return }
		if !admin.SafeMutation(r) { http.Error(w, "invalid request", http.StatusForbidden); return }
		var body struct { App string `json:"app"` }
		dec := json.NewDecoder(http.MaxBytesReader(w,r.Body,1024))
		dec.DisallowUnknownFields()
		if err:=dec.Decode(&body);err!=nil {http.Error(w,"bad request",http.StatusBadRequest);return}
		if body.App!="ollama" && body.App!="open-webui" {http.Error(w,"unsupported app",http.StatusBadRequest);return}
		data,_:=json.Marshal(body)
		appProxy(w,r,socket,http.MethodPost,"/internal/apps/install",data)
	})
	mux.HandleFunc("GET /internal/agent-health", func(w http.ResponseWriter, r *http.Request) {
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
	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Guard against DNS rebinding and unexpected Host headers.
			host := r.Host
			if !strings.HasPrefix(host, "127.0.0.1:") && !strings.HasPrefix(host, "localhost:") {
				http.Error(w, "unexpected host", http.StatusBadRequest)
				return
			}
			w.Header().Set("X-Content-Type-Options", "nosniff")
			mux.ServeHTTP(w, r)
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("locas-core listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}

func sendJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func appProxy(w http.ResponseWriter,r *http.Request,socket,method,path string,payload []byte) {
	ctx,cancel:=context.WithTimeout(r.Context(),5*time.Second)
	defer cancel()
	tr:=&http.Transport{DialContext:func(ctx context.Context,_,_ string)(net.Conn,error){
		return (&net.Dialer{Timeout:time.Second}).DialContext(ctx,"unix",socket)
	}}
	defer tr.CloseIdleConnections()
	req,err:=http.NewRequestWithContext(ctx,method,"http://agent"+path,strings.NewReader(string(payload)))
	if err!=nil {http.Error(w,"invalid request",500);return}
	req.Header.Set("Content-Type","application/json")
	resp,err:=(&http.Client{Transport:tr,Timeout:5*time.Second}).Do(req)
	if err!=nil {http.Error(w,"app manager unavailable",503);return}
	defer resp.Body.Close()
	w.Header().Set("Content-Type","application/json")
	w.Header().Set("Cache-Control","no-store")
	w.WriteHeader(resp.StatusCode)
	_,_=io.Copy(w,io.LimitReader(resp.Body,8192))
}
