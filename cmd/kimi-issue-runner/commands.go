package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func findRepoRoot(ctx context.Context, cwd string) (string, error) {
	out, err := commandOutput(ctx, cwd, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("find repository root: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func gitOutput(ctx context.Context, repoRoot string, args ...string) (string, error) {
	return commandOutput(ctx, repoRoot, "git", args...)
}

func commandOutput(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		return "", fmt.Errorf("%s: %w: %s", name, err, message)
	}
	return strings.TrimSpace(string(out)), nil
}

type gitRepo struct {
	root string
}

func (g gitRepo) head(ctx context.Context) (string, error) {
	return gitOutput(ctx, g.root, "rev-parse", "HEAD")
}

func (g gitRepo) status(ctx context.Context) (string, error) {
	return gitOutput(ctx, g.root, "status", "--porcelain", "--untracked-files=all")
}

func (g gitRepo) requireClean(ctx context.Context) error {
	status, err := g.status(ctx)
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("worktree is not clean; commit or stash existing changes before running:\n%s", status)
	}
	return nil
}

func (g gitRepo) cleanup(ctx context.Context, sha string) error {
	if _, err := gitOutput(ctx, g.root, "reset", "--hard", sha); err != nil {
		return fmt.Errorf("reset failed issue changes: %w", err)
	}
	if _, err := gitOutput(ctx, g.root, "clean", "-fd"); err != nil {
		return fmt.Errorf("remove untracked issue changes: %w", err)
	}
	return nil
}

func (g gitRepo) stageAndCheck(ctx context.Context) error {
	if _, err := gitOutput(ctx, g.root, "add", "-A"); err != nil {
		return fmt.Errorf("stage issue changes: %w", err)
	}
	if _, err := gitOutput(ctx, g.root, "diff", "--cached", "--check"); err != nil {
		return fmt.Errorf("git diff --cached --check: %w", err)
	}
	staged, err := gitOutput(ctx, g.root, "diff", "--cached", "--name-only")
	if err != nil {
		return fmt.Errorf("list staged issue changes: %w", err)
	}
	if staged == "" {
		return errNoChanges
	}
	return nil
}

func (g gitRepo) commit(ctx context.Context, issueNumber int) (string, error) {
	message := fmt.Sprintf("chore(issue): complete #%d", issueNumber)
	if _, err := gitOutput(ctx, g.root, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("commit issue #%d: %w", issueNumber, err)
	}
	return g.head(ctx)
}
