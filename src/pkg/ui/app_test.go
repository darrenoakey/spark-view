package ui

import (
	"testing"
)

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
