package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeProcesses struct {
	nextPID    int
	running    map[int]state
	starts     int
	terminates int
}

func newFakeProcesses() *fakeProcesses {
	return &fakeProcesses{nextPID: 1000, running: map[int]state{}}
}

func (f *fakeProcesses) StartCaffeinate() (state, error) {
	f.starts++
	f.nextPID++
	next := state{PID: f.nextPID, Command: "caffeinate", Started: "Mon Aug 17 12:00:00 2026"}
	f.running[next.PID] = next
	return next, nil
}

func (f *fakeProcesses) IsRunning(expected state) bool {
	actual, ok := f.running[expected.PID]
	return ok && actual == expected
}

func (f *fakeProcesses) Terminate(target state) error {
	if f.IsRunning(target) {
		f.terminates++
		delete(f.running, target.PID)
	}
	return nil
}

func TestRunLifecycleIsIdempotent(t *testing.T) {
	fake := newFakeProcesses()
	m := testManager(t, fake)

	assertRun(t, m, []string{"status"}, 0, restingOutput+"\n", "")
	assertRun(t, m, []string{"on"}, 0, brewingOutput+"\n", "")
	assertRun(t, m, []string{"on"}, 0, brewingOutput+"\n", "")
	assertRun(t, m, []string{"status"}, 0, brewingOutput+"\n", "")
	assertRun(t, m, nil, 0, restingOutput+"\n", "")
	assertRun(t, m, []string{"off"}, 0, restingOutput+"\n", "")

	if fake.starts != 1 {
		t.Fatalf("starts = %d, want 1", fake.starts)
	}
	if fake.terminates != 1 {
		t.Fatalf("terminates = %d, want 1", fake.terminates)
	}
}

func TestToggleStartsWhenInactive(t *testing.T) {
	fake := newFakeProcesses()
	m := testManager(t, fake)

	assertRun(t, m, nil, 0, brewingOutput+"\n", "")

	if fake.starts != 1 {
		t.Fatalf("starts = %d, want 1", fake.starts)
	}
	if fake.terminates != 0 {
		t.Fatalf("terminates = %d, want 0", fake.terminates)
	}
}

func TestStatusRemovesStaleState(t *testing.T) {
	fake := newFakeProcesses()
	m := testManager(t, fake)
	writeTestState(t, m.statePath, state{PID: 42, Command: "caffeinate", Started: "Mon Aug 17 11:00:00 2026"})

	assertRun(t, m, []string{"status"}, 0, restingOutput+"\n", "")

	if _, err := os.Stat(m.statePath); !os.IsNotExist(err) {
		t.Fatalf("state file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestOffDoesNotTerminateReusedPID(t *testing.T) {
	fake := newFakeProcesses()
	m := testManager(t, fake)
	stale := state{PID: 42, Command: "caffeinate", Started: "Mon Aug 17 11:00:00 2026"}
	writeTestState(t, m.statePath, stale)
	fake.running[42] = state{PID: 42, Command: "caffeinate", Started: "Mon Aug 17 11:05:00 2026"}

	assertRun(t, m, []string{"off"}, 0, restingOutput+"\n", "")

	if fake.terminates != 0 {
		t.Fatalf("terminates = %d, want 0", fake.terminates)
	}
	if _, ok := fake.running[42]; !ok {
		t.Fatal("reused pid process was removed")
	}
	if _, err := os.Stat(m.statePath); !os.IsNotExist(err) {
		t.Fatalf("state file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestInvalidCommandExitsTwo(t *testing.T) {
	fake := newFakeProcesses()
	m := testManager(t, fake)

	stdout, stderr, code := runBuffers(m, []string{"brew"})
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "invalid command: brew") {
		t.Fatalf("stderr = %q, want invalid command", stderr)
	}
}

func TestHelpAndVersion(t *testing.T) {
	fake := newFakeProcesses()
	m := testManager(t, fake)

	stdout, stderr, code := runBuffers(m, []string{"--help"})
	if code != 0 || stderr != "" {
		t.Fatalf("help code/stderr = %d/%q, want 0/empty", code, stderr)
	}
	if !strings.Contains(stdout, "coffee on") || !strings.Contains(stdout, brewingOutput) {
		t.Fatalf("help output missing command or status text: %q", stdout)
	}

	assertRun(t, m, []string{"--version"}, 0, "coffee "+version+"\n", "")
}

func testManager(t *testing.T, fake *fakeProcesses) manager {
	t.Helper()
	return manager{statePath: filepath.Join(t.TempDir(), "coffee.pid"), processes: fake}
}

func assertRun(t *testing.T, m manager, args []string, wantCode int, wantStdout string, wantStderr string) {
	t.Helper()
	stdout, stderr, code := runBuffers(m, args)
	if code != wantCode {
		t.Fatalf("code = %d, want %d", code, wantCode)
	}
	if stdout != wantStdout {
		t.Fatalf("stdout = %q, want %q", stdout, wantStdout)
	}
	if stderr != wantStderr {
		t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
	}
}

func runBuffers(m manager, args []string) (stdout string, stderr string, code int) {
	var out bytes.Buffer
	var err bytes.Buffer
	code = run(args, &out, &err, m)
	return out.String(), err.String(), code
}

func writeTestState(t *testing.T, path string, current state) {
	t.Helper()
	content, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
