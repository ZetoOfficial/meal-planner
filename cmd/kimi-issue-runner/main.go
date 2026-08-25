package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

type options struct {
	dryRun       bool
	resume       bool
	issueNumbers string
	stateDir     string
	kimiBinary   string
}

func main() {
	os.Exit(realMain())
}

func realMain() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := runCLI(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "kimi-issue-runner:", err)
		return 1
	}
	return 0
}

func runCLI(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	opts, err := parseOptions(args, stderr)
	if err != nil {
		return err
	}

	issueNumbers, err := parseIssueNumbers(opts.issueNumbers)
	if err != nil {
		return err
	}
	if opts.resume && len(issueNumbers) > 0 {
		return errors.New("--resume cannot be combined with --issues")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	repoRoot, err := findRepoRoot(ctx, cwd)
	if err != nil {
		return err
	}
	stateDir, err := resolveStateDir(ctx, repoRoot, opts.stateDir)
	if err != nil {
		return err
	}

	runner := &runner{
		repoRoot:   repoRoot,
		stateDir:   stateDir,
		kimiBinary: opts.kimiBinary,
		stdout:     stdout,
		stderr:     stderr,
	}

	if opts.resume {
		return runner.resume(ctx, opts.dryRun)
	}
	return runner.start(ctx, issueNumbers, opts.dryRun)
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	var opts options
	fs := flag.NewFlagSet("kimi-issue-runner", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&opts.dryRun, "dry-run", false, "show the selected issue queue without running Kimi")
	fs.BoolVar(&opts.resume, "resume", false, "continue the queue saved on disk")
	fs.StringVar(&opts.issueNumbers, "issues", "", "comma-separated issue numbers; order is preserved")
	fs.StringVar(&opts.stateDir, "state-dir", "", "state directory (default: <git-dir>/kimi-issue-runner)")
	fs.StringVar(&opts.kimiBinary, "kimi", "kimi", "Kimi Code executable")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage:")
		fmt.Fprintln(fs.Output(), "  kimi-issue-runner --dry-run [--issues 12,15,18]")
		fmt.Fprintln(fs.Output(), "  kimi-issue-runner [--issues 12,15,18]")
		fmt.Fprintln(fs.Output(), "  kimi-issue-runner --resume [--dry-run]")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Without --issues, all open issues are queued by ascending issue number.")
		fmt.Fprintln(fs.Output(), "A non-dry run requires a clean worktree and exclusive use of the checkout.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	return opts, nil
}

func resolveStateDir(ctx context.Context, repoRoot, configured string) (string, error) {
	gitDir, err := gitOutput(ctx, repoRoot, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	gitDir = filepath.Clean(gitDir)

	stateDir := configured
	if stateDir == "" {
		stateDir = filepath.Join(gitDir, "kimi-issue-runner")
	} else if !filepath.IsAbs(stateDir) {
		stateDir = filepath.Join(repoRoot, stateDir)
	}
	stateDir, err = filepath.Abs(stateDir)
	if err != nil {
		return "", fmt.Errorf("resolve state directory: %w", err)
	}
	stateDir = filepath.Clean(stateDir)

	if pathWithin(repoRoot, stateDir) && !pathWithin(gitDir, stateDir) {
		return "", fmt.Errorf("state directory %s is inside the worktree; use the default Git directory or a path outside the repository", stateDir)
	}
	return stateDir, nil
}

func pathWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
