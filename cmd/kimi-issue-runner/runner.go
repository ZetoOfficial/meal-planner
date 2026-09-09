package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const cleanupTimeout = 30 * time.Second

var errNoChanges = errors.New("issue produced no changes")

type runner struct {
	repoRoot   string
	stateDir   string
	kimiBinary string
	ghBinary   string
	stdout     io.Writer
	stderr     io.Writer
}

func (r *runner) start(ctx context.Context, issueNumbers []int, dryRun bool) error {
	if r.ghBinary == "" {
		r.ghBinary = "gh"
	}
	if !dryRun {
		if _, err := os.Stat(filepath.Join(r.stateDir, "state.json")); err == nil {
			return fmt.Errorf("state already exists at %s; use --resume", r.stateDir)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect state: %w", err)
		}
	}
	github := githubIssues{repoRoot: r.repoRoot, ghBinary: r.ghBinary}
	selected, err := github.selectIssues(ctx, issueNumbers)
	if err != nil {
		return err
	}
	if dryRun {
		printQueue(r.stdout, selected, "Selected issues (execution order)")
		return nil
	}

	git := gitRepo{root: r.repoRoot}
	if err := git.requireClean(ctx); err != nil {
		return err
	}
	if _, err := exec.LookPath(r.kimiBinary); err != nil {
		return fmt.Errorf("find Kimi executable %q: %w", r.kimiBinary, err)
	}

	state := newQueueState(r.repoRoot, selected)
	if err := r.persist(&state); err != nil {
		return err
	}
	return r.runQueue(ctx, &state)
}

func (r *runner) resume(ctx context.Context, dryRun bool) error {
	state, err := loadState(r.stateDir)
	if err != nil {
		return err
	}
	if dryRun {
		remaining := make([]issue, 0)
		for index := range state.Issues {
			run := &state.Issues[index]
			if run.Status == statusPending || run.Status == statusRunning {
				remaining = append(remaining, run.Issue)
			}
		}
		printQueue(r.stdout, remaining, "Remaining issues (execution order)")
		return nil
	}
	if err := r.recoverInterrupted(ctx, &state); err != nil {
		return err
	}
	if _, err := exec.LookPath(r.kimiBinary); err != nil {
		return fmt.Errorf("find Kimi executable %q: %w", r.kimiBinary, err)
	}
	if err := (gitRepo{root: r.repoRoot}).requireClean(ctx); err != nil {
		return err
	}
	return r.runQueue(ctx, &state)
}

func (r *runner) runQueue(ctx context.Context, state *queueState) error {
	for index := range state.Issues {
		run := &state.Issues[index]
		if run.Status != statusPending {
			continue
		}
		if err := r.runIssue(ctx, state, run); err != nil {
			return err
		}
	}

	done, unsuccessful := countResults(state)
	fmt.Fprintf(r.stdout, "\nQueue finished: %d done, %d failed/blocked. Report: %s\n", done, unsuccessful, filepath.Join(r.stateDir, "report.md"))
	if unsuccessful > 0 {
		return fmt.Errorf("queue finished with %d failed or blocked issue(s)", unsuccessful)
	}
	return nil
}

func (r *runner) runIssue(ctx context.Context, state *queueState, run *issueRun) error {
	startSHA, attemptRun, err := r.beginIssue(ctx, state, run)
	if err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "\n[%d/%d] Issue #%d: %s\n", queuePosition(state, run.Issue.Number), len(state.Issues), run.Issue.Number, run.Issue.Title)
	resultPath := filepath.Join(r.stateDir, filepath.FromSlash(attemptRun.ResultFile))
	stdoutPath := filepath.Join(r.stateDir, filepath.FromSlash(attemptRun.StdoutLog))
	stderrPath := filepath.Join(r.stateDir, filepath.FromSlash(attemptRun.StderrLog))
	if err := os.Remove(resultPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale Kimi result: %w", err)
	}

	kimiErr := r.invokeKimi(ctx, run.Issue, resultPath, stdoutPath, stderrPath)
	if ctx.Err() != nil {
		return r.handleInterruption(ctx, state, run, ctx.Err())
	}
	if kimiErr != nil {
		return r.finishUnsuccessful(ctx, state, run, statusFailed, fmt.Sprintf("Kimi exited unsuccessfully: %v", kimiErr), nil)
	}
	return r.processKimiResult(ctx, state, run, startSHA, resultPath)
}

func (r *runner) beginIssue(ctx context.Context, state *queueState, run *issueRun) (string, attempt, error) {
	git := gitRepo{root: r.repoRoot}
	if err := git.requireClean(ctx); err != nil {
		return "", attempt{}, fmt.Errorf("before issue #%d: %w", run.Issue.Number, err)
	}
	startSHA, err := git.head(ctx)
	if err != nil {
		return "", attempt{}, err
	}

	attemptNumber := len(run.Attempts) + 1
	issueNumber := strconv.Itoa(run.Issue.Number)
	issueDir := filepath.Join(r.stateDir, "issues", issueNumber)
	if err := os.MkdirAll(issueDir, directoryMode); err != nil {
		return "", attempt{}, fmt.Errorf("create issue log directory: %w", err)
	}
	attemptRun := attempt{
		Number:     attemptNumber,
		Status:     statusRunning,
		StartSHA:   startSHA,
		StdoutLog:  filepath.ToSlash(filepath.Join("issues", issueNumber, fmt.Sprintf("attempt-%d.stdout.log", attemptNumber))),
		StderrLog:  filepath.ToSlash(filepath.Join("issues", issueNumber, fmt.Sprintf("attempt-%d.stderr.log", attemptNumber))),
		ResultFile: filepath.ToSlash(filepath.Join("issues", issueNumber, fmt.Sprintf("attempt-%d.result.json", attemptNumber))),
		StartedAt:  time.Now().UTC(),
	}
	run.Status = statusRunning
	run.Error = ""
	run.Attempts = append(run.Attempts, attemptRun)
	if err := r.persist(state); err != nil {
		return "", attempt{}, err
	}
	return startSHA, attemptRun, nil
}

func (r *runner) processKimiResult(ctx context.Context, state *queueState, run *issueRun, startSHA, resultPath string) error {
	result, resultErr := loadKimiResult(resultPath)
	if resultErr != nil {
		return r.finishUnsuccessful(ctx, state, run, statusFailed, resultErr.Error(), nil)
	}
	if result.Status != "completed" {
		status := statusFailed
		if result.Status == "blocked" {
			status = statusBlocked
		}
		return r.finishUnsuccessful(ctx, state, run, status, normalizedReason(result.Summary), result.Checks)
	}
	return r.commitCompletedIssue(ctx, state, run, startSHA, result)
}

func (r *runner) commitCompletedIssue(ctx context.Context, state *queueState, run *issueRun, startSHA string, result kimiResult) error {
	git := gitRepo{root: r.repoRoot}
	currentSHA, err := git.head(ctx)
	if err != nil {
		return err
	}
	if currentSHA != startSHA {
		return r.stopUnsafe(state, run, fmt.Sprintf("HEAD changed from %s to %s while Kimi was running; manual recovery required", shortSHA(startSHA), shortSHA(currentSHA)))
	}
	status, err := git.status(ctx)
	if err != nil {
		return err
	}
	if status == "" {
		return r.finishUnsuccessful(ctx, state, run, statusFailed, errNoChanges.Error(), result.Checks)
	}
	if err := git.stageAndCheck(ctx); err != nil {
		return r.finishUnsuccessful(ctx, state, run, statusFailed, err.Error(), result.Checks)
	}
	result.Checks = append(result.Checks, check{Command: "git diff --cached --check", Status: "passed"})

	commitSHA, err := git.commit(ctx, run.Issue.Number)
	if err != nil {
		return r.finishUnsuccessful(ctx, state, run, statusFailed, err.Error(), result.Checks)
	}
	if err := r.cleanAfterCommit(ctx, commitSHA); err != nil {
		run.CommitSHA = commitSHA
		return r.stopUnsafe(state, run, err.Error())
	}

	now := time.Now().UTC()
	run.Status = statusDone
	run.Summary = result.Summary
	run.Error = ""
	run.CommitSHA = commitSHA
	run.Checks = result.Checks
	run.FinishedAt = &now
	last := &run.Attempts[len(run.Attempts)-1]
	last.Status = statusDone
	last.FinishedAt = &now
	if err := r.persist(state); err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "Issue #%d DONE: %s\n", run.Issue.Number, shortSHA(commitSHA))
	return nil
}

func (r *runner) invokeKimi(ctx context.Context, selected issue, resultPath, stdoutPath, stderrPath string) error {
	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return fmt.Errorf("create Kimi stdout log: %w", err)
	}
	defer stdoutFile.Close()
	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		return fmt.Errorf("create Kimi stderr log: %w", err)
	}
	defer stderrFile.Close()

	prompt := buildPrompt(selected, resultPath)
	// The binary and arguments are passed directly to exec without a shell.
	// #nosec G204 -- the configurable binary is intentional and never shell-expanded.
	cmd := exec.CommandContext(ctx, r.kimiBinary, "--auto", "--output-format", "text", "--prompt", prompt)
	cmd.Dir = r.repoRoot
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("KIMI_ISSUE_NUMBER=%d", selected.Number),
		"KIMI_ISSUE_RESULT_FILE="+resultPath,
	)
	cmd.Stdout = io.MultiWriter(r.stdout, stdoutFile)
	cmd.Stderr = io.MultiWriter(r.stderr, stderrFile)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (r *runner) finishUnsuccessful(ctx context.Context, state *queueState, run *issueRun, status issueStatus, reason string, checks []check) error {
	last := &run.Attempts[len(run.Attempts)-1]
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	currentSHA, err := (gitRepo{root: r.repoRoot}).head(cleanupCtx)
	if err != nil {
		return r.stopUnsafe(state, run, fmt.Sprintf("cannot verify HEAD before rollback: %v", err))
	}
	if currentSHA != last.StartSHA {
		return r.stopUnsafe(state, run, fmt.Sprintf("HEAD changed from %s to %s; refusing automatic rollback", shortSHA(last.StartSHA), shortSHA(currentSHA)))
	}
	if err := (gitRepo{root: r.repoRoot}).cleanup(cleanupCtx, last.StartSHA); err != nil {
		return r.stopUnsafe(state, run, err.Error())
	}

	now := time.Now().UTC()
	run.Status = status
	run.Summary = reason
	run.Error = reason
	run.Checks = checks
	run.FinishedAt = &now
	last.Status = status
	last.Error = reason
	last.FinishedAt = &now
	if err := r.persist(state); err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "Issue #%d %s: %s\n", run.Issue.Number, status, reason)
	return nil
}

func (r *runner) handleInterruption(ctx context.Context, state *queueState, run *issueRun, cause error) error {
	last := &run.Attempts[len(run.Attempts)-1]
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	currentSHA, err := (gitRepo{root: r.repoRoot}).head(cleanupCtx)
	if err != nil || currentSHA != last.StartSHA {
		return r.stopUnsafe(state, run, "runner was interrupted and HEAD changed; manual recovery required")
	}
	if err := (gitRepo{root: r.repoRoot}).cleanup(cleanupCtx, last.StartSHA); err != nil {
		return r.stopUnsafe(state, run, fmt.Sprintf("runner was interrupted and cleanup failed: %v", err))
	}

	now := time.Now().UTC()
	run.Status = statusPending
	run.Error = "interrupted; this issue will be retried on --resume"
	last.Status = statusFailed
	last.Error = run.Error
	last.FinishedAt = &now
	if err := r.persist(state); err != nil {
		return err
	}
	return cause
}

func (r *runner) stopUnsafe(state *queueState, run *issueRun, reason string) error {
	now := time.Now().UTC()
	run.Status = statusBlocked
	run.Summary = reason
	run.Error = reason
	run.FinishedAt = &now
	if len(run.Attempts) > 0 {
		last := &run.Attempts[len(run.Attempts)-1]
		last.Status = statusBlocked
		last.Error = reason
		last.FinishedAt = &now
	}
	if err := r.persist(state); err != nil {
		return errors.Join(errors.New(reason), err)
	}
	return errors.New(reason)
}

func (r *runner) recoverInterrupted(ctx context.Context, state *queueState) error {
	git := gitRepo{root: r.repoRoot}
	for index := range state.Issues {
		run := &state.Issues[index]
		if run.Status != statusRunning {
			continue
		}
		if len(run.Attempts) == 0 {
			return r.stopUnsafe(state, run, fmt.Sprintf("issue #%d is RUNNING without attempt metadata", run.Issue.Number))
		}
		last := &run.Attempts[len(run.Attempts)-1]
		currentSHA, err := git.head(ctx)
		if err != nil {
			return err
		}
		if currentSHA != last.StartSHA {
			return r.stopUnsafe(state, run, fmt.Sprintf("cannot resume issue #%d: HEAD changed from %s to %s", run.Issue.Number, shortSHA(last.StartSHA), shortSHA(currentSHA)))
		}
		if err := git.cleanup(ctx, last.StartSHA); err != nil {
			return r.stopUnsafe(state, run, fmt.Sprintf("recover issue #%d: %v", run.Issue.Number, err))
		}
		now := time.Now().UTC()
		last.Status = statusFailed
		last.Error = "recovered interrupted attempt"
		last.FinishedAt = &now
		run.Status = statusPending
		run.Error = ""
		run.FinishedAt = nil
		fmt.Fprintf(r.stdout, "Recovered interrupted issue #%d; it will be retried in a fresh Kimi session.\n", run.Issue.Number)
	}
	return r.persist(state)
}

func (r *runner) cleanAfterCommit(ctx context.Context, commitSHA string) error {
	git := gitRepo{root: r.repoRoot}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	status, err := git.status(cleanupCtx)
	if err != nil {
		return err
	}
	if status == "" {
		return nil
	}
	return git.cleanup(cleanupCtx, commitSHA)
}

func (r *runner) persist(state *queueState) error {
	if err := saveState(r.stateDir, state); err != nil {
		return err
	}
	if err := writeReport(r.stateDir, *state); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

func printQueue(out io.Writer, issues []issue, heading string) {
	fmt.Fprintf(out, "%s: %d\n", heading, len(issues))
	for index, selected := range issues {
		fmt.Fprintf(out, "%d. #%d %s\n", index+1, selected.Number, selected.Title)
	}
}

func queuePosition(state *queueState, issueNumber int) int {
	for index := range state.Issues {
		if state.Issues[index].Issue.Number == issueNumber {
			return index + 1
		}
	}
	return 0
}

func countResults(state *queueState) (done, unsuccessful int) {
	for index := range state.Issues {
		switch state.Issues[index].Status {
		case statusDone:
			done++
		case statusFailed, statusBlocked:
			unsuccessful++
		case statusPending, statusRunning:
			// Not finished yet, so neither column counts it.
		}
	}
	return done, unsuccessful
}

func normalizedReason(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Kimi did not provide a reason"
	}
	return value
}
