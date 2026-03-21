package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// Tracker manages session counts for containers via lock files in ~/.devc/sessions/.
type Tracker struct {
	dir string
}

// NewTracker creates a session tracker. Creates the sessions directory if needed.
func NewTracker() (*Tracker, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(home, ".devc", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	return &Tracker{dir: dir}, nil
}

type sessionData struct {
	PIDs []int `json:"pids"`
}

func (t *Tracker) sessionFile(containerName string) string {
	return filepath.Join(t.dir, containerName+".json")
}

// Attach records a new session for the container. Returns the new session count.
func (t *Tracker) Attach(containerName string) (int, error) {
	data := t.load(containerName)
	data.PIDs = append(data.PIDs, os.Getpid())
	if err := t.save(containerName, data); err != nil {
		return 0, err
	}
	return len(data.PIDs), nil
}

// Detach removes the current session. Returns remaining session count.
func (t *Tracker) Detach(containerName string) (int, error) {
	data := t.load(containerName)
	pid := os.Getpid()

	filtered := make([]int, 0, len(data.PIDs))
	for _, p := range data.PIDs {
		if p != pid {
			filtered = append(filtered, p)
		}
	}
	data.PIDs = filtered

	if err := t.save(containerName, data); err != nil {
		return 0, err
	}
	return len(data.PIDs), nil
}

// Count returns the number of active sessions, pruning dead PIDs.
func (t *Tracker) Count(containerName string) int {
	data := t.load(containerName)
	alive := t.pruneDeadPIDs(data.PIDs)
	if len(alive) != len(data.PIDs) {
		data.PIDs = alive
		_ = t.save(containerName, data)
	}
	return len(alive)
}

// Clean removes the session file for a container.
func (t *Tracker) Clean(containerName string) {
	_ = os.Remove(t.sessionFile(containerName))
}

func (t *Tracker) load(containerName string) *sessionData {
	raw, err := os.ReadFile(t.sessionFile(containerName))
	if err != nil {
		return &sessionData{}
	}
	var data sessionData
	if err := json.Unmarshal(raw, &data); err != nil {
		return &sessionData{}
	}
	return &data
}

func (t *Tracker) save(containerName string, data *sessionData) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(t.sessionFile(containerName), raw, 0600)
}

func (t *Tracker) pruneDeadPIDs(pids []int) []int {
	alive := make([]int, 0, len(pids))
	for _, pid := range pids {
		if isProcessAlive(pid) {
			alive = append(alive, pid)
		}
	}
	return alive
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// FormatCount returns a human-readable session count string.
func FormatCount(n int) string {
	if n == 0 {
		return "no active sessions"
	}
	return strconv.Itoa(n) + " active session" + plural(n)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
