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
		VRAMBudgetGB: 100.0,
		VRAMUsedGB:   37.0,
		Models: []Model{
			{
				ID:           "flux-schnell",
				State:        "loaded",
				MemoryGB:     32.0,
				ActiveJobs:   1,
				QueuedJobs:   3,
				MaxInstances: 2,
			},
			{
				ID:           "sonic",
				State:        "loaded",
				MemoryGB:     5.0,
				IdleSeconds:  ptrFloat(142.3),
				MaxInstances: 1,
			},
		},
		Queue: Queue{
			Queued:    4,
			Running:   1,
			Completed: 57,
			Failed:    2,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/ps", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fixture)
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })

	client := NewClient("http://" + listener.Addr().String())
	status, err := client.PS()
	if err != nil {
		t.Fatalf("PS() error: %v", err)
	}

	if status.VRAMBudgetGB != 100.0 {
		t.Errorf("VRAMBudgetGB = %v, want 100.0", status.VRAMBudgetGB)
	}
	if status.VRAMUsedGB != 37.0 {
		t.Errorf("VRAMUsedGB = %v, want 37.0", status.VRAMUsedGB)
	}
	if len(status.Models) != 2 {
		t.Fatalf("Models count = %d, want 2", len(status.Models))
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
		w.Write([]byte("internal error"))
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })

	client := NewClient("http://" + listener.Addr().String())
	_, err = client.PS()
	if err == nil {
		t.Fatal("PS() should return error for 500 status")
	}
}

// TestSetMaxInstances verifies the client sends a correct PATCH request.
func TestSetMaxInstances(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	var gotModelID string
	var gotMax int

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /v1/models/{id}", func(w http.ResponseWriter, r *http.Request) {
		gotModelID = r.PathValue("id")
		var body map[string]int
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		gotMax = body["max_instances"]
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{
			"max_instances":          gotMax,
			"previous_max_instances": 1,
		})
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })

	client := NewClient("http://" + listener.Addr().String())
	err = client.SetMaxInstances("flux-schnell", 5)
	if err != nil {
		t.Fatalf("SetMaxInstances() error: %v", err)
	}
	if gotModelID != "flux-schnell" {
		t.Errorf("model ID = %q, want %q", gotModelID, "flux-schnell")
	}
	if gotMax != 5 {
		t.Errorf("max_instances = %d, want 5", gotMax)
	}
}

// TestSetMaxInstancesError verifies the client handles server errors.
func TestSetMaxInstancesError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /v1/models/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("model not found"))
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })

	client := NewClient("http://" + listener.Addr().String())
	err = client.SetMaxInstances("nonexistent", 3)
	if err == nil {
		t.Fatal("SetMaxInstances() should return error for 404 status")
	}
}

func ptrFloat(v float64) *float64 { return &v }
