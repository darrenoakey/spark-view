package ui

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"sparkview/pkg/arbiter"
)

// TestProgressFraction verifies the progress-bar fraction is clamped and safe.
func TestProgressFraction(t *testing.T) {
	tests := []struct {
		name        string
		done, total int
		want        float64
	}{
		{"zero total", 0, 0, 0},
		{"half", 5, 10, 0.5},
		{"the ledger example", 3, 28, 3.0 / 28.0},
		{"complete", 10, 10, 1},
		{"overshoot clamps", 12, 10, 1}, // total can lag done momentarily
		{"negative total", 1, -1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := progressFraction(tt.done, tt.total); got != tt.want {
				t.Errorf("progressFraction(%d,%d) = %v, want %v", tt.done, tt.total, got, tt.want)
			}
		})
	}
}

// TestInProgressETAFormatting proves the ledger's worked example renders as a
// human-readable ETA: ltx2-dev-noise1 at 6 min/action with 25 outstanding =>
// 9000s => "2h 30m".
func TestInProgressETAFormatting(t *testing.T) {
	ip := arbiter.InProgress{
		DoneSinceLoad:    3,
		TotalSinceLoad:   28,
		AvgActionSeconds: 360,
		ETASeconds:       25 * 360, // 25 outstanding * 6 min
	}
	if got := formatDuration(ip.ETASeconds); got != "2h 30m" {
		t.Errorf("ETA format = %q, want %q", got, "2h 30m")
	}
	if got := progressFraction(ip.DoneSinceLoad, ip.TotalSinceLoad); got != 3.0/28.0 {
		t.Errorf("progress = %v, want %v", got, 3.0/28.0)
	}
}

// TestGaugeColor verifies gauge color thresholds.
func TestGaugeColor(t *testing.T) {
	tests := []struct {
		name string
		pct  float64
		want string
	}{
		{"low usage", 0.3, "green"},
		{"mid usage", 0.6, "cyan"},
		{"high usage", 0.8, "orange"},
		{"full usage", 0.95, "red"},
		{"zero", 0.0, "green"},
		{"exactly 50%", 0.5, "cyan"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gaugeColor(tt.pct)
			var gotName string
			switch got {
			case gaugeLow:
				gotName = "green"
			case gaugeMid:
				gotName = "cyan"
			case gaugeHigh:
				gotName = "orange"
			case gaugeFull:
				gotName = "red"
			default:
				gotName = "unknown"
			}
			if gotName != tt.want {
				t.Errorf("gaugeColor(%v) = %s, want %s", tt.pct, gotName, tt.want)
			}
		})
	}
}

// TestFormatDuration verifies duration formatting.
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name    string
		seconds float64
		want    string
	}{
		{"seconds only", 42.0, "42s"},
		{"minutes and seconds", 142.3, "2m 22s"},
		{"hours and minutes", 3661.0, "1h 1m"},
		{"zero", 0.0, "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.seconds)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

// TestMutedIfStaleKeepsLiveColours proves a live reading is painted with its
// real accent colour — muting must cost nothing while the link is up.
func TestMutedIfStaleKeepsLiveColours(t *testing.T) {
	for _, clr := range []color.NRGBA{accentGreen, accentRed, gaugeFull, textSecondary, dotLoaded} {
		if got := mutedIfStale(clr, true); got != clr {
			t.Errorf("mutedIfStale(%v, live) = %v, want it unchanged", clr, got)
		}
	}
}

// TestMutedIfStaleGreysFrozenTelemetry is the regression guard for the
// 2026-08-12 misread: spark had been dead and off the network for 84 minutes
// while the status bar still painted "VRAM 24 (80) / 90 GB, GPU 0%" in live
// colours, which read as a running-but-broken GPU. Stale values must be
// colourless (R==G==B) and visibly darker than the live accent.
func TestMutedIfStaleGreysFrozenTelemetry(t *testing.T) {
	for _, clr := range []color.NRGBA{accentGreen, accentRed, gaugeFull, dotLoaded} {
		got := mutedIfStale(clr, false)
		if got.R != got.G || got.G != got.B {
			t.Errorf("mutedIfStale(%v, stale) = %v, want a neutral grey", clr, got)
		}
		brightest := max(max(clr.R, clr.G), clr.B)
		if got.R >= brightest {
			t.Errorf("mutedIfStale(%v, stale) = %v, want dimmer than %d", clr, got, brightest)
		}
		if got.A != clr.A {
			t.Errorf("mutedIfStale(%v, stale) alpha = %d, want %d", clr, got.A, clr.A)
		}
	}
}

// TestLastDataTextSilentBeforeFirstSuccess: with no successful poll ever there
// is no stale data, so the SERVER DOWN panel must not claim an age.
func TestLastDataTextSilentBeforeFirstSuccess(t *testing.T) {
	if got := lastDataText(time.Time{}, time.Now()); got != "" {
		t.Errorf("lastDataText(zero) = %q, want empty", got)
	}
}

// TestLastDataTextReportsRealAge pins the exact incident: last good poll at
// 17:08:11, read at 18:32 — the panel must name both the clock time and the
// 1h 24m age, because "offline" alone did not stop the frozen bar being read
// as live.
func TestLastDataTextReportsRealAge(t *testing.T) {
	lastGood := time.Date(2026, 8, 12, 17, 8, 11, 0, time.Local)
	now := lastGood.Add(84*time.Minute + 23*time.Second)

	got := lastDataText(lastGood, now)
	for _, want := range []string{"17:08:11", "1h 24m", "ago"} {
		if !strings.Contains(got, want) {
			t.Errorf("lastDataText = %q, want it to contain %q", got, want)
		}
	}
}

// TestLastDataTextNeverGoesNegative: a clock skew backwards must not render a
// negative age.
func TestLastDataTextNeverGoesNegative(t *testing.T) {
	lastGood := time.Now()
	got := lastDataText(lastGood, lastGood.Add(-5*time.Minute))
	if strings.Contains(got, "-") {
		t.Errorf("lastDataText with skewed clock = %q, want no negative age", got)
	}
}

// TestPrepareJobRows covers the live-jobs panel's data prep: fields pass
// through in server order (oldest first), state colours map per state, and an
// absent/empty panel yields no rows so the section hides entirely (the
// graceful-degradation path for older arbiters).
func TestPrepareJobRows(t *testing.T) {
	if rows := prepareJobRows(nil); rows != nil {
		t.Errorf("prepareJobRows(nil) = %v, want nil", rows)
	}
	if rows := prepareJobRows([]arbiter.ActiveJob{}); len(rows) != 0 {
		t.Errorf("prepareJobRows(empty) = %v, want empty", rows)
	}

	jobs := []arbiter.ActiveJob{
		{JobID: "j-1", Type: "ltx25-encode", Model: "ltx25", State: "running",
			Who: "waggler", Why: "hang williams video"},
		{JobID: "j-2", Type: "tts", Model: "kokoro", State: "queued"},
		{JobID: "j-3", Type: "chat", Model: "llm:gemma", State: "following", Who: "beezle"},
		{JobID: "j-4", Type: "whisper", Model: "whisper-large", State: "future-state"},
	}
	rows := prepareJobRows(jobs)
	if len(rows) != len(jobs) {
		t.Fatalf("rows = %d, want %d", len(rows), len(jobs))
	}

	r := rows[0]
	if r.State != "running" || r.Type != "ltx25-encode" || r.Model != "ltx25" ||
		r.Who != "waggler" || r.Why != "hang williams video" {
		t.Errorf("rows[0] = %+v, want the ltx encode job fields verbatim", r)
	}
	if r.StateClr != accentGreen {
		t.Errorf("rows[0].StateClr = %v, want accentGreen for running", r.StateClr)
	}
	if rows[1].StateClr != accentPurple {
		t.Errorf("rows[1].StateClr = %v, want accentPurple for queued", rows[1].StateClr)
	}
	if rows[2].StateClr != accentOrange {
		t.Errorf("rows[2].StateClr = %v, want accentOrange for following", rows[2].StateClr)
	}
	if rows[3].StateClr != textMuted {
		t.Errorf("rows[3].StateClr = %v, want textMuted for an unknown state", rows[3].StateClr)
	}

	// Optional fields stay empty — the renderer shows a dash, never a guess.
	if rows[1].Who != "" || rows[1].Why != "" {
		t.Errorf("rows[1] who/why = %q/%q, want empty", rows[1].Who, rows[1].Why)
	}
}

// TestJobStateColor pins the state→colour mapping the live-jobs rows rely on.
func TestJobStateColor(t *testing.T) {
	tests := []struct {
		state string
		want  color.NRGBA
	}{
		{"running", accentGreen},
		{"scheduled", accentCyan},
		{"queued", accentPurple},
		{"following", accentOrange},
		{"", textMuted},
		{"something-new", textMuted},
	}
	for _, tt := range tests {
		if got := jobStateColor(tt.state); got != tt.want {
			t.Errorf("jobStateColor(%q) = %v, want %v", tt.state, got, tt.want)
		}
	}
}

// TestJobDetailLine covers the hover-reveal line: who and why joined in full,
// whichever is present alone, and an explicit placeholder when the job carries
// no provenance at all.
func TestJobDetailLine(t *testing.T) {
	tests := []struct {
		name string
		job  jobRowData
		want string
	}{
		{"who and why", jobRowData{Who: "waggler", Why: "hang williams video"}, "waggler — hang williams video"},
		{"why only", jobRowData{Why: "hang williams video"}, "hang williams video"},
		{"who only", jobRowData{Who: "beezle"}, "beezle"},
		{"neither", jobRowData{}, "(no provenance recorded)"},
	}
	for _, tt := range tests {
		if got := jobDetailLine(tt.job); got != tt.want {
			t.Errorf("%s: jobDetailLine = %q, want %q", tt.name, got, tt.want)
		}
	}
}
