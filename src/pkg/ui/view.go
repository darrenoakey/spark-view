package ui

import (
	"fmt"
	"math"
	"time"

	"sparkview/pkg/arbiter"
)

// view is everything the dashboard paints, derived ONCE per poll (on the poller
// goroutine, never the GUI thread) from the /v1/ps response plus the connection
// state.
//
// It holds only values that actually reach the screen, already rounded or
// formatted to display precision. Two views are therefore equal exactly when
// the rendered frame would be identical, which is what lets Refresh skip the
// redraw entirely while the server is idle: fields the UI never shows
// (idle_seconds, loaded_at, job ids, timestamps…) tick on every poll and must
// not be able to force a repaint, and sub-GB VRAM jitter that rounds to the
// same label must not either.
type view struct {
	polled    bool   // at least one poll has finished (false => "Connecting…")
	connected bool   // the most recent poll succeeded
	errText   string // the most recent poll error; empty while connected
	// lastData is the SERVER DOWN panel's "Last data 17:08:11 — 4m 5s ago"
	// line. Only set while disconnected (so it can never force a repaint while
	// the link is up); while the server is down it legitimately advances on
	// each failed poll so the age on screen stays honest.
	lastData string

	rows []rowView // one per model, server order
	bar  barView   // bottom status bar; kept (muted) while disconnected
}

// rowView is one model row exactly as displayed.
type rowView struct {
	ID            string
	State         string
	Loaded        bool // state is loaded or active
	MaxInstances  int
	MaxConcurrent int
	VRAM          string
	ActiveJobs    int
	QueuedJobs    int
	Progress      *progressView // nil unless the model is doing in-flight work
	ETA           string        // empty unless the model is doing in-flight work
	Hosts         []hostBadge
	Provenance    string // who/why badge label; empty => no badge
}

// progressView is the Progress cell: done/total since the model loaded.
type progressView struct {
	Done  int
	Total int
}

// barView is the bottom status bar exactly as displayed.
type barView struct {
	// VRAM in whole GB — the precision the label shows. The gauge is drawn
	// from the same rounded numbers so it can never disagree with the label
	// and fractional jitter can never force a repaint.
	VRAMActualGB    int
	VRAMAllocatedGB int
	VRAMBudgetGB    int
	GPUPct          int
	Remote          []remoteView
	Running         int
	Queued          int
	Completed       int
	Failed          int
}

// remoteView is one remote-host fleet dot.
type remoteView struct {
	HostID    string
	Reachable bool
}

// nextView derives the view for a poll that just finished at now.
//
// On success it is built entirely from status. On failure the previous rows
// and status bar are KEPT (the last telemetry is useful history — the UI mutes
// it so it never reads as live) and only the connection fields change.
// lastGood is the end of the most recent successful poll (zero if none).
func nextView(prev view, status arbiter.Status, err error, lastGood, now time.Time) view {
	if err != nil {
		next := prev
		next.polled = true
		next.connected = false
		next.errText = err.Error()
		next.lastData = lastDataText(lastGood, now)
		return next
	}
	return view{
		polled:    true,
		connected: true,
		rows:      buildRows(status),
		bar:       buildBar(status),
	}
}

func buildRows(status arbiter.Status) []rowView {
	prov := modelProvenance(status.ActiveJobsDetail)
	rows := make([]rowView, 0, len(status.Models))
	for _, m := range status.Models {
		loaded := m.State == "loaded" || m.State == "active"
		row := rowView{
			ID:            m.ID,
			State:         m.State,
			Loaded:        loaded,
			MaxInstances:  m.MaxInstances,
			MaxConcurrent: m.MaxConcurrent,
			VRAM:          vramLabel(m, loaded),
			ActiveJobs:    m.ActiveJobs,
			QueuedJobs:    m.QueuedJobs,
			Hosts:         runningHosts(m),
			Provenance:    provenanceLabel(prov[m.ID]),
		}
		if ip := m.InProgress; ip != nil {
			row.Progress = &progressView{Done: ip.DoneSinceLoad, Total: ip.TotalSinceLoad}
			row.ETA = "~" + formatDuration(ip.ETASeconds)
		}
		rows = append(rows, row)
	}
	return rows
}

// vramLabel is the VRAM cell text: "-" for unloaded models, otherwise the
// configured size, plus actual usage when it differs.
func vramLabel(m arbiter.Model, loaded bool) string {
	if !loaded {
		return "-"
	}
	if m.VRAMGB > 0 && m.VRAMGB != m.MemoryGB {
		return fmt.Sprintf("%.0f (%.0f)", m.VRAMGB, m.MemoryGB)
	}
	return fmt.Sprintf("%.0f GB", m.MemoryGB)
}

func buildBar(status arbiter.Status) barView {
	bar := barView{
		VRAMActualGB:    wholeGB(status.VRAMActualGB),
		VRAMAllocatedGB: wholeGB(status.VRAMAllocatedGB),
		VRAMBudgetGB:    wholeGB(status.VRAMBudgetGB),
		GPUPct:          status.GPUUtilizationPct,
		Running:         status.Queue.Running,
		Queued:          status.Queue.Queued,
		Completed:       status.Queue.Completed,
		Failed:          status.Queue.Failed,
	}
	for _, h := range status.RemoteHosts {
		bar.Remote = append(bar.Remote, remoteView{HostID: h.HostID, Reachable: h.Reachable})
	}
	return bar
}

// wholeGB rounds a GB figure to the precision the status bar displays.
func wholeGB(gb float64) int {
	return int(math.Round(gb))
}

// changedText is the status-bar clock: when the data on screen last CHANGED.
// It is deliberately a fixed clock time rather than a ticking "Ns ago" — a
// ticking label would need a repaint every second forever, which is exactly
// the idle work this dashboard must not do. Liveness is carried by the
// connection dot and the muted/"offline" treatment of a failed poll.
func changedText(changedAt time.Time) string {
	if changedAt.IsZero() {
		return ""
	}
	return "changed " + changedAt.Format("15:04:05")
}
