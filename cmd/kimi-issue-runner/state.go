package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	stateVersion    = 1
	directoryMode   = 0o755
	regularFileMode = 0o644
)

type issueStatus string

const (
	statusPending issueStatus = "PENDING"
	statusRunning issueStatus = "RUNNING"
	statusDone    issueStatus = "DONE"
	statusFailed  issueStatus = "FAILED"
	statusBlocked issueStatus = "BLOCKED"
)

type queueState struct {
	Version   int        `json:"version"`
	RepoRoot  string     `json:"repo_root"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Issues    []issueRun `json:"issues"`
}

type issueRun struct {
	Issue      issue       `json:"issue"`
	Status     issueStatus `json:"status"`
	Summary    string      `json:"summary,omitempty"`
	Error      string      `json:"error,omitempty"`
	CommitSHA  string      `json:"commit_sha,omitempty"`
	Checks     []check     `json:"checks,omitempty"`
	Attempts   []attempt   `json:"attempts,omitempty"`
	FinishedAt *time.Time  `json:"finished_at,omitempty"`
}

type attempt struct {
	Number     int         `json:"number"`
	Status     issueStatus `json:"status"`
	StartSHA   string      `json:"start_sha"`
	StdoutLog  string      `json:"stdout_log"`
	StderrLog  string      `json:"stderr_log"`
	ResultFile string      `json:"result_file"`
	StartedAt  time.Time   `json:"started_at"`
	FinishedAt *time.Time  `json:"finished_at,omitempty"`
	Error      string      `json:"error,omitempty"`
}

type issue struct {
	Number   int            `json:"number"`
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	URL      string         `json:"url"`
	State    string         `json:"state"`
	Comments []issueComment `json:"comments,omitempty"`
}

type issueComment struct {
	Author    issueAuthor `json:"author"`
	Body      string      `json:"body"`
	CreatedAt time.Time   `json:"createdAt"`
}

type issueAuthor struct {
	Login string `json:"login"`
}

type check struct {
	Command string `json:"command"`
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
}

func newQueueState(repoRoot string, issues []issue) queueState {
	now := time.Now().UTC()
	runs := make([]issueRun, 0, len(issues))
	for _, selected := range issues {
		runs = append(runs, issueRun{Issue: selected, Status: statusPending})
	}
	return queueState{
		Version:   stateVersion,
		RepoRoot:  repoRoot,
		CreatedAt: now,
		UpdatedAt: now,
		Issues:    runs,
	}
}

func loadState(stateDir string) (queueState, error) {
	path := filepath.Join(stateDir, "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return queueState{}, fmt.Errorf("read state %s: %w", path, err)
	}
	var state queueState
	if err := json.Unmarshal(data, &state); err != nil {
		return queueState{}, fmt.Errorf("decode state %s: %w", path, err)
	}
	if state.Version != stateVersion {
		return queueState{}, fmt.Errorf("unsupported state version %d", state.Version)
	}
	return state, nil
}

func saveState(stateDir string, state *queueState) error {
	if err := os.MkdirAll(stateDir, directoryMode); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	state.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	data = append(data, '\n')
	return writeFileAtomic(filepath.Join(stateDir, "state.json"), data, regularFileMode)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("set mode on temporary file for %s: %w", path, err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file for %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
