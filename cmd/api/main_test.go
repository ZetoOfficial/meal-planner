package main

import (
	"strings"
	"testing"

	"github.com/ZetoOfficial/meal-planner/internal/llm/anthropic"
	"github.com/ZetoOfficial/meal-planner/internal/llm/openai"
)

func TestLLMProviderResolvesTheBuiltInProviders(t *testing.T) {
	cases := map[string]string{
		anthropic.Name: "anthropic",
		openai.Name:    "openai",
	}
	for name, wantName := range cases {
		t.Run(name, func(t *testing.T) {
			provider, err := llmProvider(name)
			if err != nil {
				t.Fatalf("llmProvider(%q) error = %v", name, err)
			}
			if got := provider.Name(); got != wantName {
				t.Fatalf("Name() = %q, want %q", got, wantName)
			}
		})
	}
}

func TestLLMProviderRejectsAnUnknownName(t *testing.T) {
	_, err := llmProvider("does-not-exist")
	if err == nil {
		t.Fatal("llmProvider() = nil error, want an error for an unknown name")
	}
	if !strings.Contains(err.Error(), anthropic.Name) || !strings.Contains(err.Error(), openai.Name) {
		t.Fatalf("llmProvider() error = %v, want it to name both valid providers %q and %q",
			err, anthropic.Name, openai.Name)
	}
}
