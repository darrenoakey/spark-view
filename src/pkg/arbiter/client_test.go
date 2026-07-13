package arbiter

import (
	"encoding/json"
	"net"
	"net/http"
	"testing"
)

// TestPS verifies the client correctly parses a real /v1/ps response.
func TestPS(t *testing.T) {
	// Start a real HTTP server with fixture data
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	fixture := Status{
		VRAMBudgetGB:    100.0,
		VRAMUsedGB:      37.0,
		VRAMActualGB:    43.0,
		VRAMAllocatedGB: 77.0,
		VRAMFreeGB:      23.0,
		Models: []Model{
			{
				ID:            "flux-schnell",
				State:         "loaded",
				MemoryGB:      32.0,
				ActiveJobs:    1,
				QueuedJobs:    3,
				MaxInstances:  2,
				MaxConcurrent: 1,
			},
			{
				ID:            "sonic",
				State:         "loaded",
				MemoryGB:      5.0,
				IdleSeconds:   ptrFloat(142.3),
				MaxInstances:  1,
				MaxConcurrent: 4,
			},
			{
				ID:              "llm:gemma4-26b",
				State:           "active",
				MaxInstances:    1,
				MaxConcurrent:   1,
				LoadedInstances: 1,
				TotalInstances:  2,
				Instances: []Instance{
					{
						Host:       "boringstack",
						Remote:     true,
						State:      "active",
						ActiveJobs: 2,
						InstanceID: "llm:gemma4-26b@boringstack",
					},
					{
						Host:       "spark",
						Remote:     false,
						State:      "stopped",
						InstanceID: "llm:gemma4-26b",
					},
				},
			},
		},
		Queue: Queue{
			Queued:    4,
			Running:   1,
			Completed: 57,
			Failed:    2,
		},
		RemoteHosts: []RemoteHost{
			{
				HostID:       "boringstack",
				Addr:         "http://10.0.0.42:11434",
				Reachable:    true,
				ModelsLoaded: []string{"gemma4-26b-32k:latest", "llama3.2:3b"},
				UsedGB:       12.0,
				BudgetGB:     96.0,
			},
			{
				HostID:    "darrens-mbp",
				Addr:      "http://10.0.0.44:11434",
				Reachable: false,
				BudgetGB:  40.0,
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/ps", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fixture)
	})

	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	client := NewClient("http://" + listener.Addr().String())
	status, err := client.PS()
	if err != nil {
		t.Fatalf("PS() error: %v", err)
	}

	if status.VRAMBudgetGB != 100.0 {
		t.Errorf("VRAMBudgetGB = %v, want 100.0", status.VRAMBudgetGB)
	}
	if status.VRAMActualGB != 43.0 {
		t.Errorf("VRAMActualGB = %v, want 43.0", status.VRAMActualGB)
	}
	if status.VRAMAllocatedGB != 77.0 {
		t.Errorf("VRAMAllocatedGB = %v, want 77.0", status.VRAMAllocatedGB)
	}
	if status.VRAMUsedGB != 37.0 {
		t.Errorf("VRAMUsedGB = %v, want 37.0", status.VRAMUsedGB)
	}
	if len(status.Models) != 3 {
		t.Fatalf("Models count = %d, want 3", len(status.Models))
	}
	if status.Models[0].ID != "flux-schnell" {
		t.Errorf("Models[0].ID = %q, want %q", status.Models[0].ID, "flux-schnell")
	}
	if status.Models[0].ActiveJobs != 1 {
		t.Errorf("Models[0].ActiveJobs = %d, want 1", status.Models[0].ActiveJobs)
	}
	if status.Models[1].IdleSeconds == nil {
		t.Fatal("Models[1].IdleSeconds is nil, want 142.3")
	}
	if *status.Models[1].IdleSeconds != 142.3 {
		t.Errorf("Models[1].IdleSeconds = %v, want 142.3", *status.Models[1].IdleSeconds)
	}
	if status.Queue.Completed != 57 {
		t.Errorf("Queue.Completed = %d, want 57", status.Queue.Completed)
	}
	if status.Models[0].MaxInstances != 2 {
		t.Errorf("Models[0].MaxInstances = %d, want 2", status.Models[0].MaxInstances)
	}
	if status.Models[1].MaxInstances != 1 {
		t.Errorf("Models[1].MaxInstances = %d, want 1", status.Models[1].MaxInstances)
	}
	if status.Models[0].MaxConcurrent != 1 {
		t.Errorf("Models[0].MaxConcurrent = %d, want 1", status.Models[0].MaxConcurrent)
	}
	if status.Models[1].MaxConcurrent != 4 {
		t.Errorf("Models[1].MaxConcurrent = %d, want 4", status.Models[1].MaxConcurrent)
	}

	// Per-instance host info (multi-machine model).
	gemma := status.Models[2]
	if gemma.ID != "llm:gemma4-26b" {
		t.Fatalf("Models[2].ID = %q, want llm:gemma4-26b", gemma.ID)
	}
	if gemma.LoadedInstances != 1 {
		t.Errorf("gemma.LoadedInstances = %d, want 1", gemma.LoadedInstances)
	}
	if gemma.TotalInstances != 2 {
		t.Errorf("gemma.TotalInstances = %d, want 2", gemma.TotalInstances)
	}
	if len(gemma.Instances) != 2 {
		t.Fatalf("gemma.Instances count = %d, want 2", len(gemma.Instances))
	}
	if gemma.Instances[0].Host != "boringstack" {
		t.Errorf("gemma.Instances[0].Host = %q, want boringstack", gemma.Instances[0].Host)
	}
	if !gemma.Instances[0].Remote {
		t.Error("gemma.Instances[0].Remote = false, want true")
	}
	if gemma.Instances[0].State != "active" {
		t.Errorf("gemma.Instances[0].State = %q, want active", gemma.Instances[0].State)
	}
	if gemma.Instances[0].ActiveJobs != 2 {
		t.Errorf("gemma.Instances[0].ActiveJobs = %d, want 2", gemma.Instances[0].ActiveJobs)
	}
	if gemma.Instances[0].InstanceID != "llm:gemma4-26b@boringstack" {
		t.Errorf("gemma.Instances[0].InstanceID = %q, want llm:gemma4-26b@boringstack", gemma.Instances[0].InstanceID)
	}
	if gemma.Instances[1].Remote {
		t.Error("gemma.Instances[1].Remote = true, want false (spark is local)")
	}

	// Remote-hosts fleet panel.
	if len(status.RemoteHosts) != 2 {
		t.Fatalf("RemoteHosts count = %d, want 2", len(status.RemoteHosts))
	}
	rh := status.RemoteHosts[0]
	if rh.HostID != "boringstack" {
		t.Errorf("RemoteHosts[0].HostID = %q, want boringstack", rh.HostID)
	}
	if rh.Addr != "http://10.0.0.42:11434" {
		t.Errorf("RemoteHosts[0].Addr = %q, want http://10.0.0.42:11434", rh.Addr)
	}
	if !rh.Reachable {
		t.Error("RemoteHosts[0].Reachable = false, want true")
	}
	if len(rh.ModelsLoaded) != 2 {
		t.Errorf("RemoteHosts[0].ModelsLoaded count = %d, want 2", len(rh.ModelsLoaded))
	}
	if rh.BudgetGB != 96.0 {
		t.Errorf("RemoteHosts[0].BudgetGB = %v, want 96.0", rh.BudgetGB)
	}
	if rh.UsedGB != 12.0 {
		t.Errorf("RemoteHosts[0].UsedGB = %v, want 12.0", rh.UsedGB)
	}
	if status.RemoteHosts[1].Reachable {
		t.Error("RemoteHosts[1].Reachable = true, want false")
	}
}

// TestPSInProgress verifies the client parses the in_progress panel that
// arbiter emits only for actively-working models, and leaves it nil otherwise.
func TestPSInProgress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	// Raw JSON so we exercise the real wire shape arbiter produces: "busy" has an
	// in_progress block (ledger worked example), "idle" has none.
	body := `{
		"models": [
			{"id":"ltx2-dev-noise1","state":"active","active_jobs":2,"queued_jobs":23,
			 "in_progress":{"done_since_load":3,"total_since_load":28,
			   "avg_action_seconds":360,"eta_seconds":9000,"loaded_at":1700000000.5}},
			{"id":"whisper-large","state":"loaded","active_jobs":0,"queued_jobs":0}
		]
	}`

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/ps", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	client := NewClient("http://" + listener.Addr().String())
	status, err := client.PS()
	if err != nil {
		t.Fatalf("PS() error: %v", err)
	}
	if len(status.Models) != 2 {
		t.Fatalf("Models count = %d, want 2", len(status.Models))
	}

	busy := status.Models[0]
	if busy.InProgress == nil {
		t.Fatal("busy model InProgress is nil, want populated panel")
	}
	if busy.InProgress.DoneSinceLoad != 3 {
		t.Errorf("DoneSinceLoad = %d, want 3", busy.InProgress.DoneSinceLoad)
	}
	if busy.InProgress.TotalSinceLoad != 28 {
		t.Errorf("TotalSinceLoad = %d, want 28", busy.InProgress.TotalSinceLoad)
	}
	if busy.InProgress.AvgActionSeconds != 360 {
		t.Errorf("AvgActionSeconds = %v, want 360", busy.InProgress.AvgActionSeconds)
	}
	if busy.InProgress.ETASeconds != 9000 {
		t.Errorf("ETASeconds = %v, want 9000", busy.InProgress.ETASeconds)
	}
	if busy.InProgress.LoadedAt != 1700000000.5 {
		t.Errorf("LoadedAt = %v, want 1700000000.5", busy.InProgress.LoadedAt)
	}

	idle := status.Models[1]
	if idle.InProgress != nil {
		t.Errorf("idle model InProgress = %+v, want nil", idle.InProgress)
	}
}

// TestPSError verifies the client handles server errors gracefully.
func TestPSError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/ps", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	})

	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	client := NewClient("http://" + listener.Addr().String())
	_, err = client.PS()
	if err == nil {
		t.Fatal("PS() should return error for 500 status")
	}
}

// TestPatchModel verifies the client sends a correct PATCH request.
func TestPatchModel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	var gotModelID string
	var gotBody map[string]int

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /v1/models/{id}", func(w http.ResponseWriter, r *http.Request) {
		gotModelID = r.PathValue("id")
		gotBody = make(map[string]int)
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gotBody)
	})

	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	client := NewClient("http://" + listener.Addr().String())

	// Test max_instances
	err = client.PatchModel("flux-schnell", map[string]int{"max_instances": 5})
	if err != nil {
		t.Fatalf("PatchModel(max_instances) error: %v", err)
	}
	if gotModelID != "flux-schnell" {
		t.Errorf("model ID = %q, want %q", gotModelID, "flux-schnell")
	}
	if gotBody["max_instances"] != 5 {
		t.Errorf("max_instances = %d, want 5", gotBody["max_instances"])
	}

	// Test max_concurrent
	err = client.PatchModel("sonic", map[string]int{"max_concurrent": 8})
	if err != nil {
		t.Fatalf("PatchModel(max_concurrent) error: %v", err)
	}
	if gotModelID != "sonic" {
		t.Errorf("model ID = %q, want %q", gotModelID, "sonic")
	}
	if gotBody["max_concurrent"] != 8 {
		t.Errorf("max_concurrent = %d, want 8", gotBody["max_concurrent"])
	}
}

// TestPatchModelError verifies the client handles server errors.
func TestPatchModelError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /v1/models/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("model not found"))
	})

	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	client := NewClient("http://" + listener.Addr().String())
	err = client.PatchModel("nonexistent", map[string]int{"max_instances": 3})
	if err == nil {
		t.Fatal("PatchModel() should return error for 404 status")
	}
}

// TestClearJobs verifies the client sends correct DELETE requests for
// both "queue" and "running" scopes.
func TestClearJobs(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	var gotPath string

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /v1/models/{id}/{scope}", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model_id":          r.PathValue("id"),
			"cancelled_queued":  3,
			"cancelled_running": 1,
		})
	})

	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	client := NewClient("http://" + listener.Addr().String())

	// Test queue scope
	err = client.ClearJobs("flux-schnell", "queue")
	if err != nil {
		t.Fatalf("ClearJobs(queue) error: %v", err)
	}
	if gotPath != "/v1/models/flux-schnell/queue" {
		t.Errorf("path = %q, want /v1/models/flux-schnell/queue", gotPath)
	}

	// Test running scope (clears all)
	err = client.ClearJobs("sonic", "running")
	if err != nil {
		t.Fatalf("ClearJobs(running) error: %v", err)
	}
	if gotPath != "/v1/models/sonic/running" {
		t.Errorf("path = %q, want /v1/models/sonic/running", gotPath)
	}
}

func ptrFloat(v float64) *float64 { return &v }
