package ui

import (
	"errors"
	"testing"
	"time"

	"sparkview/pkg/arbiter"
)

func ptr(f float64) *float64 { return &f }

// busyStatus is a realistic /v1/ps snapshot: one model working on a remote
// host with a progress bar and provenance, one idle-but-loaded model, one
// unloaded model, and a remote host in the fleet bar.
func busyStatus() arbiter.Status {
	started := 1760300000.5
	return arbiter.Status{
		VRAMBudgetGB:      90.0,
		VRAMActualGB:      24.31,
		VRAMAllocatedGB:   80.2,
		GPUUtilizationPct: 37,
		Models: []arbiter.Model{
			{
				ID: "ltx25", State: "active", MemoryGB: 40, VRAMGB: 38.6,
				ActiveJobs: 1, QueuedJobs: 2, MaxInstances: 1, MaxConcurrent: 1,
				Instances: []arbiter.Instance{{Host: "spark", State: "active", ActiveJobs: 1}},
				InProgress: &arbiter.InProgress{
					DoneSinceLoad: 3, TotalSinceLoad: 28,
					AvgActionSeconds: 360, ETASeconds: 9000, LoadedAt: 1760290000.25,
				},
			},
			{
				ID: "kokoro", State: "loaded", MemoryGB: 2, IdleSeconds: ptr(12.5),
				MaxInstances: 1, MaxConcurrent: 4,
				Instances: []arbiter.Instance{{Host: "boringstack", Remote: true, State: "loaded"}},
			},
			{ID: "flux-schnell", State: "unloaded", MemoryGB: 32, MaxInstances: 2, MaxConcurrent: 1},
		},
		Queue:       arbiter.Queue{Queued: 2, Running: 1, Completed: 590, Failed: 3},
		RemoteHosts: []arbiter.RemoteHost{{HostID: "boringstack", Reachable: true, UsedGB: 12.4, BudgetGB: 96}},
		ActiveJobsDetail: []arbiter.ActiveJob{
			{JobID: "j-1", Model: "ltx25", State: "running", CreatedAt: 1760299990.1, StartedAt: &started,
				Who: "waggler", Why: "hang williams video"},
		},
	}
}

// TestIdleTickingFieldsDoNotRepaint is the core CPU guard: between two polls of
// an otherwise idle server, the fields Arbiter rewrites every time (idle
// seconds, sub-display-precision VRAM jitter, job ids and timestamps, remote
// host usage, average action time) must NOT count as a change — a change is
// what triggers a full GPU repaint.
func TestIdleTickingFieldsDoNotRepaint(t *testing.T) {
	var a App
	t0 := time.Date(2026, 10, 13, 7, 0, 0, 0, time.Local)
	if _, _, changed := a.apply(busyStatus(), nil, t0); !changed {
		t.Fatal("first successful poll must paint")
	}

	s := busyStatus()
	s.Models[1].IdleSeconds = ptr(17.5)
	s.Models[0].InProgress.AvgActionSeconds = 361.7
	s.Models[0].InProgress.LoadedAt = 1760290001
	s.VRAMActualGB = 24.44   // still "24"
	s.VRAMAllocatedGB = 79.9 // still "80"
	s.VRAMUsedGB = 55.5      // never displayed
	s.VRAMFreeGB = 10.1      // never displayed
	s.RemoteHosts[0].UsedGB = 13.9
	s.RemoteHosts[0].ModelsLoaded = []string{"llama3.2:3b"}
	s.ActiveJobsDetail[0].JobID = "j-1-renamed"
	s.ActiveJobsDetail[0].CreatedAt = 1760299999

	t1 := t0.Add(5 * time.Second)
	if _, _, changed := a.apply(s, nil, t1); changed {
		t.Fatal("poll differing only in undisplayed/sub-precision fields must not repaint")
	}
	if !a.changedAt.Equal(t0) {
		t.Errorf("changedAt = %v, want it left at the first paint %v", a.changedAt, t0)
	}
	if !a.lastGood.Equal(t1) {
		t.Errorf("lastGood = %v, want %v (every successful poll refreshes it)", a.lastGood, t1)
	}
}

// TestVisibleChangesRepaint proves every field the user can see still repaints.
func TestVisibleChangesRepaint(t *testing.T) {
	mutations := map[string]func(*arbiter.Status){
		"active jobs":      func(s *arbiter.Status) { s.Models[0].ActiveJobs = 2 },
		"queued jobs":      func(s *arbiter.Status) { s.Models[0].QueuedJobs = 5 },
		"model state":      func(s *arbiter.Status) { s.Models[2].State = "loading" },
		"max instances":    func(s *arbiter.Status) { s.Models[2].MaxInstances = 3 },
		"max concurrent":   func(s *arbiter.Status) { s.Models[1].MaxConcurrent = 2 },
		"vram label":       func(s *arbiter.Status) { s.Models[0].VRAMGB = 35.2 },
		"progress":         func(s *arbiter.Status) { s.Models[0].InProgress.DoneSinceLoad = 4 },
		"eta":              func(s *arbiter.Status) { s.Models[0].InProgress.ETASeconds = 8640 },
		"finished working": func(s *arbiter.Status) { s.Models[0].InProgress = nil },
		"host moved":       func(s *arbiter.Status) { s.Models[1].Instances[0].Host = "macmini" },
		"why":              func(s *arbiter.Status) { s.ActiveJobsDetail[0].Why = "other video" },
		"new model":        func(s *arbiter.Status) { s.Models = append(s.Models, arbiter.Model{ID: "new"}) },
		"bar vram":         func(s *arbiter.Status) { s.VRAMActualGB = 25.6 },
		"gpu pct":          func(s *arbiter.Status) { s.GPUUtilizationPct = 38 },
		"queue done":       func(s *arbiter.Status) { s.Queue.Completed = 591 },
		"queue failed":     func(s *arbiter.Status) { s.Queue.Failed = 4 },
		"remote down":      func(s *arbiter.Status) { s.RemoteHosts[0].Reachable = false },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var a App
			t0 := time.Now()
			a.apply(busyStatus(), nil, t0)
			s := busyStatus()
			mutate(&s)
			t1 := t0.Add(5 * time.Second)
			if _, _, changed := a.apply(s, nil, t1); !changed {
				t.Fatalf("visible change %q did not repaint", name)
			}
			if !a.changedAt.Equal(t1) {
				t.Errorf("changedAt = %v, want %v", a.changedAt, t1)
			}
		})
	}
}

// TestDisconnectKeepsLastDataAndRepaints: losing the server repaints once into
// SERVER DOWN with the last telemetry kept (muted) and its true age, and
// reconnecting repaints back.
func TestDisconnectKeepsLastDataAndRepaints(t *testing.T) {
	var a App
	t0 := time.Date(2026, 8, 12, 17, 8, 11, 0, time.Local)
	a.apply(busyStatus(), nil, t0)

	down := errors.New("connect: no route to host")
	t1 := t0.Add(84*time.Minute + 23*time.Second)
	prev, next, changed := a.apply(arbiter.Status{}, down, t1)
	if !changed || !prev.connected || next.connected {
		t.Fatalf("disconnect: changed=%v prev.connected=%v next.connected=%v", changed, prev.connected, next.connected)
	}
	if next.errText != down.Error() {
		t.Errorf("errText = %q, want %q", next.errText, down.Error())
	}
	if len(next.rows) != 3 || next.bar.Completed != 590 {
		t.Errorf("last telemetry must be kept while down; rows=%d completed=%d", len(next.rows), next.bar.Completed)
	}
	if want := lastDataText(t0, t1); next.lastData != want {
		t.Errorf("lastData = %q, want %q", next.lastData, want)
	}

	if _, next, changed := a.apply(busyStatus(), nil, t1.Add(5*time.Second)); !changed || !next.connected || next.lastData != "" {
		t.Fatalf("reconnect: changed=%v connected=%v lastData=%q", changed, next.connected, next.lastData)
	}
}

// TestStartupUnreachableShowsNoAge: failing before any success leaves no
// stale data to describe.
func TestStartupUnreachableShowsNoAge(t *testing.T) {
	var a App
	_, next, changed := a.apply(arbiter.Status{}, errors.New("refused"), time.Now())
	if !changed || !next.polled || next.connected || next.lastData != "" || len(next.rows) != 0 {
		t.Fatalf("startup failure view = %+v (changed=%v)", next, changed)
	}
}

func TestBuildRowsDisplayValues(t *testing.T) {
	rows := buildRows(busyStatus())
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	ltx := rows[0]
	if !ltx.Loaded || ltx.VRAM != "39 (40)" || ltx.ETA != "~2h 30m" ||
		ltx.Progress == nil || *ltx.Progress != (progressView{Done: 3, Total: 28}) ||
		ltx.Provenance != "hang williams video" {
		t.Errorf("ltx25 row = %+v", ltx)
	}
	if len(ltx.Hosts) != 1 || ltx.Hosts[0] != (hostBadge{host: "spark"}) {
		t.Errorf("ltx25 hosts = %+v", ltx.Hosts)
	}
	kokoro := rows[1]
	if kokoro.VRAM != "2 GB" || kokoro.Progress != nil || kokoro.ETA != "" || kokoro.Provenance != "" ||
		len(kokoro.Hosts) != 1 || kokoro.Hosts[0] != (hostBadge{host: "boringstack", remote: true}) {
		t.Errorf("kokoro row = %+v", kokoro)
	}
	if flux := rows[2]; flux.Loaded || flux.VRAM != "-" || flux.Hosts != nil {
		t.Errorf("flux row = %+v", flux)
	}

	bar := buildBar(busyStatus())
	if bar.VRAMActualGB != 24 || bar.VRAMAllocatedGB != 80 || bar.VRAMBudgetGB != 90 || bar.GPUPct != 37 ||
		len(bar.Remote) != 1 || bar.Remote[0] != (remoteView{HostID: "boringstack", Reachable: true}) {
		t.Errorf("bar = %+v", bar)
	}
}

func TestChangedText(t *testing.T) {
	if got := changedText(time.Time{}); got != "" {
		t.Errorf("changedText(zero) = %q, want empty", got)
	}
	at := time.Date(2026, 10, 13, 7, 41, 5, 0, time.Local)
	if got := changedText(at); got != "changed 07:41:05" {
		t.Errorf("changedText = %q, want %q", got, "changed 07:41:05")
	}
}
