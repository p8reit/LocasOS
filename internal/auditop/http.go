package auditop

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/p8reit/LocasOS/internal/auditlog"
)

const maxResponse = 8 * 1024 * 1024

type response struct {
	header   http.Header
	body     bytes.Buffer
	status   int
	overflow bool
}

func (r *response) Header() http.Header { return r.header }
func (r *response) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
}
func (r *response) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = 200
	}
	if r.body.Len()+len(data) > maxResponse {
		r.overflow = true
		return 0, errors.New("audited response exceeds limit")
	}
	return r.body.Write(data)
}

// Serve buffers the response until its outcome is durable. Audit failures never
// release successful cookies/bodies. Effects already performed may be uncertain.
func Serve(recorder auditlog.Recorder, service string, state *State, w http.ResponseWriter, r *http.Request, next http.Handler) {
	w.Header().Set("X-Locas-Correlation-ID", state.Attribution.CorrelationID)
	w.Header().Set("Cache-Control", "no-store")
	if err := Record(recorder, state.Event(service, "intent", "pending", 0)); err != nil {
		w.Header().Del("Set-Cookie")
		http.Error(w, "Security audit unavailable; operation was not started.", http.StatusServiceUnavailable)
		return
	}
	buffer := &response{header: make(http.Header)}
	// Recover only the handler boundary. Never format or log a panic value: it
	// may contain credentials, prompts, or other private handler data.
	panicked := true
	func() {
		defer func() { _ = recover() }()
		next.ServeHTTP(buffer, r.WithContext(With(r.Context(), state)))
		panicked = false
	}()
	if panicked {
		buffer.status = http.StatusServiceUnavailable
	}
	if buffer.status == 0 {
		buffer.status = 200
	}
	outcome := state.Outcome
	if outcome == "" {
		switch {
		case buffer.status == 202:
			outcome = "accepted"
		case buffer.status < 400:
			outcome = "succeeded"
		case buffer.status < 500:
			outcome = "denied"
		default:
			outcome = "failed"
		}
	}
	if buffer.overflow || panicked {
		outcome = "uncertain"
	}
	if err := Record(recorder, state.Event(service, "result", outcome, buffer.status)); err != nil || buffer.overflow || panicked {
		// Nothing from the handler, including any session cookie, has been released.
		w.Header().Del("Set-Cookie")
		http.Error(w, "Audit outcome unavailable; the operation may have completed. Check status before retrying.", http.StatusServiceUnavailable)
		return
	}
	for key, values := range buffer.header {
		w.Header()[key] = append([]string(nil), values...)
	}
	w.Header().Set("X-Locas-Correlation-ID", state.Attribution.CorrelationID)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(buffer.status)
	_, _ = w.Write(buffer.body.Bytes())
}
