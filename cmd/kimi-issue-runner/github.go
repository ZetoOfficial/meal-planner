package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type githubIssues struct {
	repoRoot string
	ghBinary string
}

func (g githubIssues) selectIssues(ctx context.Context, numbers []int) ([]issue, error) {
	if len(numbers) == 0 {
		args := []string{"issue", "list", "--state", "open", "--limit", "10000", "--json", "number,title,body,url,state,comments"}
		selected, err := g.run(ctx, args...)
		if err != nil {
			return nil, err
		}
		sort.Slice(selected, func(i, j int) bool { return selected[i].Number < selected[j].Number })
		return selected, nil
	}

	selected := make([]issue, 0, len(numbers))
	for _, number := range numbers {
		args := []string{"issue", "view", strconv.Itoa(number), "--json", "number,title,body,url,state,comments"}
		issues, err := g.run(ctx, args...)
		if err != nil {
			return nil, err
		}
		if len(issues) != 1 {
			return nil, fmt.Errorf("gh returned %d records for issue #%d", len(issues), number)
		}
		selected = append(selected, issues[0])
	}
	return selected, nil
}

func (g githubIssues) run(ctx context.Context, args ...string) ([]issue, error) {
	out, err := commandOutput(ctx, g.repoRoot, g.ghBinary, args...)
	if err != nil {
		return nil, fmt.Errorf("load GitHub issues: %w", err)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		var one issue
		if err := json.Unmarshal([]byte(out), &one); err != nil {
			return nil, fmt.Errorf("decode GitHub issue: %w", err)
		}
		return []issue{one}, nil
	}
	var many []issue
	if err := json.Unmarshal([]byte(out), &many); err != nil {
		return nil, fmt.Errorf("decode GitHub issue list: %w", err)
	}
	return many, nil
}

func parseIssueNumbers(value string) ([]int, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	numbers := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		number, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || number <= 0 {
			return nil, fmt.Errorf("invalid issue number %q", part)
		}
		if _, exists := seen[number]; exists {
			return nil, fmt.Errorf("duplicate issue number #%d", number)
		}
		seen[number] = struct{}{}
		numbers = append(numbers, number)
	}
	return numbers, nil
}
