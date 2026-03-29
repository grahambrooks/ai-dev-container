package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatCount(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "no active sessions"},
		{1, "1 active session"},
		{3, "3 active sessions"},
	}
	for _, tt := range tests {
		got := FormatCount(tt.n)
		if got != tt.want {
			t.Errorf("FormatCount(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestPlural(t *testing.T) {
	if plural(1) != "" {
		t.Error("plural(1) should be empty")
	}
	if plural(0) != "s" {
		t.Error("plural(0) should be 's'")
	}
	if plural(5) != "s" {
		t.Error("plural(5) should be 's'")
	}
}

func newTestTracker(t *testing.T) *Tracker {
	t.Helper()
	dir := t.TempDir()
	return &Tracker{dir: dir}
}

func TestTracker_AttachDetach(t *testing.T) {
	tr := newTestTracker(t)

	count, err := tr.Attach("test-container")
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if count != 1 {
		t.Errorf("Attach count = %d, want 1", count)
	}

	// Verify file was created
	f := tr.sessionFile("test-container")
	if _, err := os.Stat(f); os.IsNotExist(err) {
		t.Error("session file was not created")
	}

	// Detach
	remaining, err := tr.Detach("test-container")
	if err != nil {
		t.Fatalf("Detach: %v", err)
	}
	if remaining != 0 {
		t.Errorf("Detach remaining = %d, want 0", remaining)
	}
}

func TestTracker_Count(t *testing.T) {
	tr := newTestTracker(t)

	// Empty container has 0 sessions
	if c := tr.Count("empty"); c != 0 {
		t.Errorf("Count(empty) = %d, want 0", c)
	}

	// Attach current process
	_, _ = tr.Attach("test-container")
	c := tr.Count("test-container")
	if c != 1 {
		t.Errorf("Count after attach = %d, want 1", c)
	}
}

func TestTracker_CountPrunesDeadPIDs(t *testing.T) {
	tr := newTestTracker(t)

	// Write a session file with a dead PID
	data := &sessionData{PIDs: []int{999999999}}
	if err := tr.save("dead-pids", data); err != nil {
		t.Fatal(err)
	}

	c := tr.Count("dead-pids")
	if c != 0 {
		t.Errorf("Count with dead PIDs = %d, want 0", c)
	}
}

func TestTracker_Clean(t *testing.T) {
	tr := newTestTracker(t)

	_, _ = tr.Attach("cleanup-test")
	f := tr.sessionFile("cleanup-test")
	if _, err := os.Stat(f); os.IsNotExist(err) {
		t.Fatal("session file should exist before clean")
	}

	tr.Clean("cleanup-test")
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Error("session file should be removed after clean")
	}
}

func TestTracker_MultipleAttach(t *testing.T) {
	tr := newTestTracker(t)

	c1, _ := tr.Attach("multi")
	c2, _ := tr.Attach("multi")
	if c1 != 1 {
		t.Errorf("first attach = %d, want 1", c1)
	}
	if c2 != 2 {
		t.Errorf("second attach = %d, want 2", c2)
	}
}

func TestTracker_SessionFile(t *testing.T) {
	tr := newTestTracker(t)
	f := tr.sessionFile("my-container")
	expected := filepath.Join(tr.dir, "my-container.json")
	if f != expected {
		t.Errorf("sessionFile = %q, want %q", f, expected)
	}
}

func TestTracker_LoadMissingFile(t *testing.T) {
	tr := newTestTracker(t)
	data := tr.load("nonexistent")
	if len(data.PIDs) != 0 {
		t.Errorf("load missing file should return empty PIDs, got %v", data.PIDs)
	}
}

func TestTracker_LoadCorruptFile(t *testing.T) {
	tr := newTestTracker(t)
	f := tr.sessionFile("corrupt")
	os.WriteFile(f, []byte("not json"), 0600)

	data := tr.load("corrupt")
	if len(data.PIDs) != 0 {
		t.Errorf("load corrupt file should return empty PIDs, got %v", data.PIDs)
	}
}

func TestIsProcessAlive(t *testing.T) {
	// Current process should be alive
	if !isProcessAlive(os.Getpid()) {
		t.Error("current process should be alive")
	}
	// Invalid PIDs
	if isProcessAlive(0) {
		t.Error("PID 0 should not be alive")
	}
	if isProcessAlive(-1) {
		t.Error("negative PID should not be alive")
	}
	// Very high PID unlikely to exist
	if isProcessAlive(999999999) {
		t.Error("PID 999999999 should not be alive")
	}
}

func TestPruneDeadPIDs(t *testing.T) {
	tr := newTestTracker(t)
	myPID := os.Getpid()
	pids := []int{myPID, 999999999, 999999998}

	alive := tr.pruneDeadPIDs(pids)
	if len(alive) != 1 || alive[0] != myPID {
		t.Errorf("pruneDeadPIDs = %v, want [%d]", alive, myPID)
	}
}

func TestTracker_DetachNonexistent(t *testing.T) {
	tr := newTestTracker(t)
	remaining, err := tr.Detach("nonexistent")
	if err != nil {
		t.Fatalf("Detach nonexistent: %v", err)
	}
	if remaining != 0 {
		t.Errorf("remaining = %d, want 0", remaining)
	}
}
