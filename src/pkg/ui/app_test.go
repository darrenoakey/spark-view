package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"
)

// TestIsNoRouteToHost verifies the classifier matches routing-level failures
// through the exact wrapper chain the arbiter client produces, and rejects
// server-down / timeout errors that the app recovers from on its own.
func TestIsNoRouteToHost(t *testing.T) {
	// Mimic net/http: url.Error -> net.OpError -> os.SyscallError -> Errno,
	// then the client's fmt.Errorf("arbiter ps request: %w", ...).
	wrap := func(errno syscall.Errno) error {
		op := &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", errno)}
		ue := &url.Error{Op: "Get", URL: "http://10.0.0.254:8400/v1/ps", Err: op}
		return fmt.Errorf("arbiter ps request: %w", ue)
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"no route to host", wrap(syscall.EHOSTUNREACH), true},
		{"network unreachable", wrap(syscall.ENETUNREACH), true},
		{"connection refused", wrap(syscall.ECONNREFUSED), false},
		{"timeout", fmt.Errorf("arbiter ps request: %w", context.DeadlineExceeded), false},
		{"decode error", errors.New("arbiter ps decode: unexpected EOF"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNoRouteToHost(tt.err); got != tt.want {
				t.Errorf("isNoRouteToHost(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
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
