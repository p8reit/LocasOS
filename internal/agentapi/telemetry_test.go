package agentapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/p8reit/LocasOS/internal/gpu"
	"github.com/p8reit/LocasOS/internal/hardware"
)

func TestTelemetry(t *testing.T) {
	handler := NewTelemetryHandler(
		func() (hardware.Snapshot, error) { return hardware.Snapshot{Node: "node-1", CPUs: 8}, nil },
		func(context.Context) ([]gpu.Device, error) {
			return []gpu.Device{{Name: "RTX 3060", MemoryTotalMiB: 12288}}, nil
		},
	)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/telemetry", nil))
	if w.Code != http.StatusOK { t.Fatalf("status %d", w.Code) }
	var data Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil { t.Fatal(err) }
	if data.Host.Node != "node-1" || len(data.GPUs) != 1 || data.GPUStatus != "available" { t.Fatalf("%+v", data) }
}
func TestUnavailableGPU(t *testing.T) {
	handler := NewTelemetryHandler(
		func() (hardware.Snapshot, error) { return hardware.Snapshot{Node: "node-1"}, nil },
		func(context.Context) ([]gpu.Device, error) { return nil, errors.New("no driver") },
	)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/telemetry", nil))
	if w.Code != http.StatusOK { t.Fatalf("status %d", w.Code) }
	var data Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil { t.Fatal(err) }
	if data.GPUStatus != "unavailable" || len(data.GPUs) != 0 { t.Fatalf("%+v", data) }
}
func TestHostFailure(t *testing.T) {
	handler := NewTelemetryHandler(
		func() (hardware.Snapshot, error) { return hardware.Snapshot{}, errors.New("host failed") },
		func(context.Context) ([]gpu.Device, error) { t.Fatal("GPU called after host failure"); return nil, nil },
	)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/telemetry", nil))
	if w.Code != http.StatusServiceUnavailable { t.Fatalf("status %d", w.Code) }
}
func TestRejectOtherMethods(t *testing.T) {
	h := NewTelemetryHandler(func()(hardware.Snapshot,error){return hardware.Snapshot{},nil}, func(context.Context)([]gpu.Device,error){return nil,nil})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/internal/telemetry", nil))
	if w.Code != http.StatusMethodNotAllowed { t.Fatalf("status %d", w.Code) }
}
