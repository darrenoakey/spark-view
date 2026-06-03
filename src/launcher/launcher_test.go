// Package launcher_test exercises the supervising launcher script that keeps the
// Spark View GUI alive across crashes, Gio teardown panics, and intermittent
// macOS EDEADLK exec failures.
package launcher_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// launcherPath returns the absolute path to the launcher script under test.
func launcherPath(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	path := filepath.Join(wd, "sparkview-launcher")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("launcher script not found at %s: %v", path, err)
	}
	return path
}

// writeFakeBinary creates an executable that appends one line to counterPath each
// time it runs, then exits with the given code. It simulates the GUI process.
func writeFakeBinary(t *testing.T, dir, counterPath string, exitCode int) string {
	t.Helper()
	bin := filepath.Join(dir, "fake-sparkview")
	script := "#!/bin/bash\n" +
		"echo run >> " + strconv.Quote(counterPath) + "\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return bin
}

func countRuns(t *testing.T, counterPath string) int {
	t.Helper()
	data, err := os.ReadFile(counterPath)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "run\n")
}

// TestLauncherRelaunchesOnCrash is the core guarantee: when the GUI process exits
// non-zero (a crash / Gio panic), the launcher must bring it back. We let it run
// for a short window and assert it launched the binary multiple times.
func TestLauncherRelaunchesOnCrash(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "runs.txt")
	bin := writeFakeBinary(t, dir, counter, 1) // exit 1 == crash

	cmd := exec.Command(launcherPath(t))
	cmd.Env = append(os.Environ(),
		"SPARKVIEW_BIN="+bin,
		"SPARKVIEW_RELAUNCH_DELAY=0.2",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start launcher: %v", err)
	}
	t.Cleanup(func() {
		// Kill the whole process group so no stray children survive the test.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_, _ = cmd.Process.Wait()
	})

	// With a 0.2s delay, within ~2s we should see several relaunches.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if countRuns(t, counter) >= 3 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if got := countRuns(t, counter); got < 3 {
		t.Fatalf("expected launcher to relaunch the crashing binary >=3 times, got %d", got)
	}
}

// TestLauncherRelaunchesOnCleanExit verifies a clean exit (user closed the window)
// is also brought back, matching the keep-alive intent.
func TestLauncherRelaunchesOnCleanExit(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "runs.txt")
	bin := writeFakeBinary(t, dir, counter, 0) // exit 0 == clean close

	cmd := exec.Command(launcherPath(t))
	cmd.Env = append(os.Environ(),
		"SPARKVIEW_BIN="+bin,
		"SPARKVIEW_RELAUNCH_DELAY=0.2",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start launcher: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_, _ = cmd.Process.Wait()
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if countRuns(t, counter) >= 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if got := countRuns(t, counter); got < 2 {
		t.Fatalf("expected launcher to relaunch after a clean exit, got %d runs", got)
	}
}

// TestLauncherSurvivesMissingBinary ensures a missing/not-yet-built binary does not
// crash the launcher or spin hot — it should keep looping so it recovers once the
// binary appears.
func TestLauncherSurvivesMissingBinary(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "runs.txt")
	missing := filepath.Join(dir, "does-not-exist-yet")

	cmd := exec.Command(launcherPath(t))
	cmd.Env = append(os.Environ(),
		"SPARKVIEW_BIN="+missing,
		"SPARKVIEW_RELAUNCH_DELAY=0.2",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start launcher: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_, _ = cmd.Process.Wait()
	})

	// Let it loop a few times against the missing binary, then drop a real one in.
	time.Sleep(700 * time.Millisecond)
	bin := writeFakeBinary(t, dir, counter, 0)
	if bin != missing {
		// Replace the missing path with the working binary at the SAME path.
		if err := os.Rename(bin, missing); err != nil {
			t.Fatalf("install binary: %v", err)
		}
		if err := os.Chmod(missing, 0o755); err != nil {
			t.Fatalf("chmod: %v", err)
		}
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if countRuns(t, counter) >= 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if got := countRuns(t, counter); got < 1 {
		t.Fatalf("launcher did not recover once the binary appeared, got %d runs", got)
	}
}

// TestLauncherTerminatesOnSigterm verifies the launcher exits promptly on SIGTERM
// (so `auto stop` shuts it down cleanly rather than the manager having to SIGKILL).
func TestLauncherTerminatesOnSigterm(t *testing.T) {
	dir := t.TempDir()
	// A binary that sleeps, so there is a live child to terminate.
	bin := filepath.Join(dir, "sleeper")
	if err := os.WriteFile(bin, []byte("#!/bin/bash\nsleep 30\n"), 0o755); err != nil {
		t.Fatalf("write sleeper: %v", err)
	}

	cmd := exec.Command(launcherPath(t))
	cmd.Env = append(os.Environ(),
		"SPARKVIEW_BIN="+bin,
		"SPARKVIEW_RELAUNCH_DELAY=0.2",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start launcher: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	})

	// Give it a moment to spawn the child, then SIGTERM the launcher.
	time.Sleep(500 * time.Millisecond)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		// Exited promptly — good.
	case <-time.After(5 * time.Second):
		t.Fatal("launcher did not exit within 5s of SIGTERM")
	}
}
