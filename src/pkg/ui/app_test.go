package ui

import (
	"testing"

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
