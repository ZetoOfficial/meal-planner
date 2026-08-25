package main

import (
	"fmt"
	"strings"
)

func buildPrompt(selected issue, resultPath string) string {
	var comments strings.Builder
	if len(selected.Comments) == 0 {
		comments.WriteString("(no comments)\n")
	} else {
		for _, comment := range selected.Comments {
			fmt.Fprintf(&comments, "\n--- comment by @%s at %s ---\n%s\n", comment.Author.Login, comment.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), comment.Body)
		}
	}

	return fmt.Sprintf(`Implement GitHub Issue #%d in the current repository.

Read and obey the repository instructions (including AGENTS.md and CLAUDE.md) before editing. Inspect the existing implementation and tests. Keep the change strictly scoped to the issue. Run the relevant repository checks after implementation.

Runner safety rules:
- Do not create commits, amend commits, switch branches, reset, clean, merge, push, open pull requests, deploy, access production, or perform destructive infrastructure operations.
- Work only in the current repository and use the existing toolchain.
- Do not implement unrelated backlog issues.
- This is a fresh Kimi session; do not assume context outside the issue and repository.

When finished, write a JSON object to this exact path (it is intentionally outside the worktree):
%s

The JSON schema is:
{
  "status": "completed" | "failed" | "blocked",
  "summary": "short factual summary",
  "checks": [
    {"command": "exact command that was actually run", "status": "passed" | "failed", "details": "optional short details"}
  ]
}

Use status "completed" only when the issue is fully implemented and every reported check passed. A completed result must contain at least one check; use `+"`git diff --check`"+` as the minimum useful check when no more specific check applies. If work cannot be completed, write "failed" or "blocked" and explain why. Do not claim checks that were not run.

<github_issue>
Number: #%d
Title: %s
URL: %s
State: %s

Body:
%s

Comments:
%s</github_issue>
`, selected.Number, resultPath, selected.Number, selected.Title, selected.URL, selected.State, selected.Body, comments.String())
}
