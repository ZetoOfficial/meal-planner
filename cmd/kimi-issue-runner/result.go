package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type kimiResult struct {
	Status  string  `json:"status"`
	Summary string  `json:"summary"`
	Checks  []check `json:"checks"`
}

func loadKimiResult(path string) (kimiResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return kimiResult{}, fmt.Errorf("read Kimi result: %w", err)
	}
	defer file.Close()

	var result kimiResult
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&result); err != nil {
		return kimiResult{}, fmt.Errorf("decode Kimi result: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return kimiResult{}, errors.New("decode Kimi result: expected one JSON object")
	}

	switch result.Status {
	case "completed":
		if len(result.Checks) == 0 {
			return kimiResult{}, errors.New("completed Kimi result has no checks")
		}
		for _, check := range result.Checks {
			if check.Command == "" {
				return kimiResult{}, errors.New("kimi result contains a check without a command")
			}
			if check.Status != "passed" {
				return kimiResult{}, fmt.Errorf("completed Kimi result contains non-passing check %q", check.Command)
			}
		}
	case "failed", "blocked":
	default:
		return kimiResult{}, fmt.Errorf("kimi result has invalid status %q", result.Status)
	}
	return result, nil
}
