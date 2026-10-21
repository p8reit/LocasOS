package agentapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/p8reit/LocasOS/internal/gpu"
	"github.com/p8reit/LocasOS/internal/hardware"
)

type GPUProbe func(context.Context) ([]gpu.Device, error)
type HostProbe func() (hardware.Snapshot, error)

type Snapshot struct {
	Host      hardware.Snapshot `json:"host"`
	GPUs      []gpu.Device      `json:"gpus"`
	GPUStatus string            `json:"gpu_status"`
}

func NewTelemetryHandler(host HostProbe, gpus GPUProbe) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /internal/telemetry", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()

		h, err := host()
		if err != nil {
			http.Error(w, "host inventory unavailable", http.StatusServiceUnavailable)
			return
		}

		devices, gpuErr := gpus(ctx)
		status := "available"
		if gpuErr != nil {
			status = "unavailable"
			devices = []gpu.Device{}
		} else if devices == nil {
			devices = []gpu.Device{}
		}
		h.GPUs = nil // Do not duplicate GPU payload in the host snapshot.
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(Snapshot{Host: h, GPUs: devices, GPUStatus: status})
	})
	return mux
}
