package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

const shortSHALength = 12

func writeReport(stateDir string, state queueState) error {
	var report strings.Builder
	report.WriteString("# Kimi Issue Runner Report\n\n")
	fmt.Fprintf(&report, "Updated: %s\n\n", state.UpdatedAt.Format("2006-01-02 15:04:05Z"))
	report.WriteString("| Issue | Status | Commit | Checks | Summary | Logs |\n")
	report.WriteString("|---|---|---|---|---|---|\n")
	for index := range state.Issues {
		run := &state.Issues[index]
		commit := "—"
		if run.CommitSHA != "" {
			commit = run.CommitSHA
		}
		checks := formatChecks(run.Checks)
		summary := run.Summary
		if summary == "" {
			summary = run.Error
		}
		logs := "—"
		if len(run.Attempts) > 0 {
			last := run.Attempts[len(run.Attempts)-1]
			logs = fmt.Sprintf("[%s](%s), [%s](%s)",
				"stdout", filepath.ToSlash(last.StdoutLog),
				"stderr", filepath.ToSlash(last.StderrLog),
			)
		}
		issueLink := fmt.Sprintf("[#%d](%s) %s", run.Issue.Number, run.Issue.URL, run.Issue.Title)
		fmt.Fprintf(&report, "| %s | %s | %s | %s | %s | %s |\n",
			markdownCell(issueLink), run.Status, commit, markdownCell(checks), markdownCell(summary), logs)
	}

	return writeFileAtomic(filepath.Join(stateDir, "report.md"), []byte(report.String()), regularFileMode)
}

func formatChecks(checks []check) string {
	if len(checks) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(checks))
	for _, check := range checks {
		parts = append(parts, fmt.Sprintf("%s: %s", check.Command, check.Status))
	}
	return strings.Join(parts, "; ")
}

func markdownCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", "<br>")
	return value
}

func shortSHA(sha string) string {
	if len(sha) <= shortSHALength {
		return sha
	}
	return sha[:shortSHALength]
}
