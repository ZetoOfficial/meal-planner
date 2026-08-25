package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseIssueNumbersPreservesOrder(t *testing.T) {
	got, err := parseIssueNumbers("8, 3,21")
	if err != nil {
		t.Fatalf("parseIssueNumbers() error = %v", err)
	}
	want := []int{8, 3, 21}
	if len(got) != len(want) {
		t.Fatalf("parseIssueNumbers() = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("parseIssueNumbers() = %v, want %v", got, want)
		}
	}
}

func TestParseIssueNumbersRejectsDuplicates(t *testing.T) {
	if _, err := parseIssueNumbers("4,4"); err == nil {
		t.Fatal("parseIssueNumbers() error = nil, want duplicate error")
	}
}

func TestParseOptionsHelp(t *testing.T) {
	var output bytes.Buffer
	if _, err := parseOptions([]string{"--help"}, &output); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("parseOptions(--help) error = %v, want flag.ErrHelp", err)
	}
	if !strings.Contains(output.String(), "order is preserved") {
		t.Fatalf("help output = %q", output.String())
	}
}

func TestGitHubSelectionSortsBacklogAndPreservesExplicitOrder(t *testing.T) {
	tempDir := t.TempDir()
	gh := writeExecutable(t, tempDir, "fake-gh", `#!/bin/sh
if [ "$2" = "list" ]; then
  printf '%s\n' '[{"number":9,"title":"nine","body":"b9","url":"u9","state":"OPEN"},{"number":2,"title":"two","body":"b2","url":"u2","state":"OPEN"}]'
  exit 0
fi
number="$3"
printf '{"number":%s,"title":"issue %s","body":"body %s","url":"u%s","state":"OPEN"}\n' "$number" "$number" "$number" "$number"
`)
	client := githubIssues{repoRoot: tempDir, ghBinary: gh}

	backlog, err := client.selectIssues(context.Background(), nil)
	if err != nil {
		t.Fatalf("selectIssues(backlog) error = %v", err)
	}
	if backlog[0].Number != 2 || backlog[1].Number != 9 {
		t.Fatalf("backlog order = #%d, #%d; want #2, #9", backlog[0].Number, backlog[1].Number)
	}

	explicit, err := client.selectIssues(context.Background(), []int{9, 2})
	if err != nil {
		t.Fatalf("selectIssues(explicit) error = %v", err)
	}
	if explicit[0].Number != 9 || explicit[1].Number != 2 {
		t.Fatalf("explicit order = #%d, #%d; want #9, #2", explicit[0].Number, explicit[1].Number)
	}
}

func TestLoadKimiResultRequiresPassingChecksForCompleted(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "result.json")
	writeTestFile(t, path, `{"status":"completed","summary":"done","checks":[]}`)

	if _, err := loadKimiResult(path); err == nil {
		t.Fatal("loadKimiResult() error = nil, want validation error")
	}

	writeTestFile(t, path, `{"status":"completed","summary":"done","checks":[{"command":"go test ./...","status":"passed"}]}`)
	result, err := loadKimiResult(path)
	if err != nil {
		t.Fatalf("loadKimiResult() error = %v", err)
	}
	if result.Status != "completed" || len(result.Checks) != 1 {
		t.Fatalf("loadKimiResult() = %+v", result)
	}
}

func TestBuildPromptContainsFullIssueAndSafetyContract(t *testing.T) {
	selected := issue{
		Number: 7,
		Title:  "add planner",
		Body:   "complete body",
		URL:    "https://example.test/issues/7",
		State:  "OPEN",
		Comments: []issueComment{{
			Author:    issueAuthor{Login: "owner"},
			Body:      "important clarification",
			CreatedAt: time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
		}},
	}
	prompt := buildPrompt(selected, "/tmp/result.json")
	for _, want := range []string{"complete body", "important clarification", "/tmp/result.json", "Do not create commits", "Run the relevant repository checks"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q", want)
		}
	}
}

func TestRunnerRollsBackFailureAndCommitsNextIssue(t *testing.T) {
	repoRoot := initTestRepo(t)
	stateDir := filepath.Join(t.TempDir(), "state")
	kimi := writeExecutable(t, t.TempDir(), "fake-kimi", `#!/bin/sh
case "$KIMI_ISSUE_NUMBER" in
  101)
    printf 'partial\n' > failed.txt
    printf 'failed stdout\n'
    printf 'failed stderr\n' >&2
    printf '%s\n' '{"status":"failed","summary":"deliberate failure","checks":[{"command":"go test ./...","status":"failed"}]}' > "$KIMI_ISSUE_RESULT_FILE"
    ;;
  102)
    printf 'completed\n' > completed.txt
    printf 'completed stdout\n'
    printf 'completed stderr\n' >&2
    printf '%s\n' '{"status":"completed","summary":"implemented second issue","checks":[{"command":"go test ./...","status":"passed"}]}' > "$KIMI_ISSUE_RESULT_FILE"
    ;;
  *)
    exit 2
    ;;
esac
`)
	state := newQueueState(repoRoot, []issue{
		{Number: 101, Title: "fails", Body: "first body", URL: "u101", State: "OPEN"},
		{Number: 102, Title: "works", Body: "second body", URL: "u102", State: "OPEN"},
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	r := &runner{
		repoRoot:   repoRoot,
		stateDir:   stateDir,
		kimiBinary: kimi,
		stdout:     &stdout,
		stderr:     &stderr,
	}
	if err := r.persist(&state); err != nil {
		t.Fatalf("persist() error = %v", err)
	}

	err := r.runQueue(context.Background(), &state)
	if err == nil || !strings.Contains(err.Error(), "1 failed or blocked") {
		t.Fatalf("runQueue() error = %v, want final unsuccessful summary", err)
	}
	if state.Issues[0].Status != statusFailed {
		t.Fatalf("issue #101 status = %s, want FAILED", state.Issues[0].Status)
	}
	if state.Issues[1].Status != statusDone {
		t.Fatalf("issue #102 status = %s, want DONE", state.Issues[1].Status)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "failed.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed issue file still exists: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(repoRoot, "completed.txt")); err != nil || string(data) != "completed\n" {
		t.Fatalf("completed issue file = %q, %v", data, err)
	}
	status, err := gitOutput(context.Background(), repoRoot, "status", "--porcelain", "--untracked-files=all")
	if err != nil || status != "" {
		t.Fatalf("git status = %q, %v; want clean", status, err)
	}
	log, err := gitOutput(context.Background(), repoRoot, "log", "-1", "--format=%s")
	if err != nil || log != "chore(issue): complete #102" {
		t.Fatalf("latest commit = %q, %v", log, err)
	}
	if state.Issues[1].CommitSHA == "" {
		t.Fatal("completed issue has no commit SHA")
	}
	assertFileContains(t, filepath.Join(stateDir, "issues", "101", "attempt-1.stdout.log"), "failed stdout")
	assertFileContains(t, filepath.Join(stateDir, "issues", "101", "attempt-1.stderr.log"), "failed stderr")
	assertFileContains(t, filepath.Join(stateDir, "issues", "102", "attempt-1.stdout.log"), "completed stdout")
	assertFileContains(t, filepath.Join(stateDir, "report.md"), "| FAILED |")
	assertFileContains(t, filepath.Join(stateDir, "report.md"), "| DONE |")
}

func TestRecoverInterruptedAttemptCleansChangesAndRetries(t *testing.T) {
	repoRoot := initTestRepo(t)
	stateDir := filepath.Join(t.TempDir(), "state")
	startSHA, err := gitOutput(context.Background(), repoRoot, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(repoRoot, "partial.txt"), "partial")
	now := time.Now().UTC()
	state := newQueueState(repoRoot, []issue{{Number: 55, Title: "interrupted"}})
	state.Issues[0].Status = statusRunning
	state.Issues[0].Attempts = []attempt{{Number: 1, Status: statusRunning, StartSHA: startSHA, StartedAt: now}}
	kimi := writeExecutable(t, t.TempDir(), "fake-kimi", `#!/bin/sh
[ "$1" = "--auto" ] || exit 3
[ "$2" = "--output-format" ] || exit 3
[ "$3" = "text" ] || exit 3
[ "$4" = "--prompt" ] || exit 3
[ "$#" = "5" ] || exit 3
printf 'resumed\n' > resumed.txt
printf '%s\n' '{"status":"completed","summary":"resumed successfully","checks":[{"command":"go test ./...","status":"passed"}]}' > "$KIMI_ISSUE_RESULT_FILE"
`)
	var stdout bytes.Buffer
	r := &runner{repoRoot: repoRoot, stateDir: stateDir, kimiBinary: kimi, stdout: &stdout, stderr: &bytes.Buffer{}}
	if err := r.persist(&state); err != nil {
		t.Fatalf("persist() error = %v", err)
	}

	if err := r.resume(context.Background(), false); err != nil {
		t.Fatalf("resume() error = %v", err)
	}
	loaded, err := loadState(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Issues[0].Status != statusDone {
		t.Fatalf("status = %s, want DONE", loaded.Issues[0].Status)
	}
	if len(loaded.Issues[0].Attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(loaded.Issues[0].Attempts))
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "partial.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial file still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "resumed.txt")); err != nil {
		t.Fatalf("resumed file missing: %v", err)
	}
	if !strings.Contains(stdout.String(), "Recovered interrupted issue #55") {
		t.Fatalf("stdout = %q, want recovery message", stdout.String())
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	runTestCommand(t, repoRoot, "git", "init", "--quiet")
	runTestCommand(t, repoRoot, "git", "config", "user.name", "Runner Test")
	runTestCommand(t, repoRoot, "git", "config", "user.email", "runner@example.test")
	writeTestFile(t, filepath.Join(repoRoot, "README.md"), "test repository\n")
	runTestCommand(t, repoRoot, "git", "add", "README.md")
	runTestCommand(t, repoRoot, "git", "commit", "--quiet", "-m", "chore: initial")
	return repoRoot
}

func writeExecutable(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	return path
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func runTestCommand(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	if _, err := commandOutput(context.Background(), dir, name, args...); err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s does not contain %q:\n%s", path, want, data)
	}
}
