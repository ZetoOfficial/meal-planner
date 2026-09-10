// Package recipe turns constraints into candidate recipe templates: it renders
// the versioned prompt, forces the model to answer through one JSON schema and
// validates every candidate before the solver may consider it. Candidates are
// candidates only; promotion to a permanent recipe happens after the dish is
// actually cooked.
package recipe

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"text/template"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/llm"
)

//go:embed prompts/recipe_generation.md
var generationPrompt string

const (
	toolName = "emit_recipe_candidates"
	// maxNoveltyTemperature is the temperature of the highest novelty dial;
	// novelty 0 stays fully deterministic (DESIGN §11).
	maxNoveltyTemperature = 0.8
	maxNoveltyLevel       = 3
	defaultCandidateCount = 3
	temperatureScale      = 100
)

// Completer is the provider call the generator needs; it exists so prompt and
// parsing can be tested without an HTTP provider.
type Completer interface {
	Generate(ctx context.Context, request llm.Request) (llm.Response, error)
}

// Input is everything the model may see. Anything not listed
// here is out of v1 scope and must not reach the prompt.
type Input struct {
	Days       int
	Count      int
	Novelty    int
	MealTypes  []mealdom.Type
	Equipment  []string
	Allergens  []string
	Exclusions []string
	RoleTags   []catalogdom.RoleTag
	Products   []catalogdom.Product
	PlanID     *int64
}

type Generator struct {
	client   Completer
	template *template.Template
}

func New(client Completer) (*Generator, error) {
	parsed, err := template.New("recipe_generation").Parse(generationPrompt)
	if err != nil {
		return nil, fmt.Errorf("parse recipe prompt: %w", err)
	}
	return &Generator{client: client, template: parsed}, nil
}

// Generate asks for candidate templates and returns them as draft recipes.
// Drafts are candidates only: the solver still decides whether any of them is
// used, and promotion to permanent happens after the dish is actually cooked.
func (g *Generator) Generate(ctx context.Context, input Input) ([]recipedom.Template, error) {
	prompt, err := g.Prompt(input)
	if err != nil {
		return nil, err
	}

	response, err := g.client.Generate(ctx, llm.Request{
		System:      "You propose candidate recipe templates and answer only through the provided tool.",
		Prompt:      prompt,
		ToolName:    toolName,
		Schema:      json.RawMessage(candidateSchema),
		Temperature: noveltyTemperature(input.Novelty),
		PlanID:      input.PlanID,
	})
	if err != nil {
		return nil, err
	}
	return parseCandidates(response.Content, input)
}

// Prompt renders the versioned prompt file for the supplied constraints.
func (g *Generator) Prompt(input Input) (string, error) {
	if input.Count <= 0 {
		input.Count = defaultCandidateCount
	}
	var rendered bytes.Buffer
	if err := g.template.Execute(&rendered, input); err != nil {
		return "", fmt.Errorf("render recipe prompt: %w", err)
	}
	return rendered.String(), nil
}

// noveltyTemperature maps the novelty dial onto provider temperature: level 0
// is deterministic, level 3 is "surprise me".
func noveltyTemperature(novelty int) float64 {
	if novelty <= 0 {
		return 0
	}
	if novelty >= maxNoveltyLevel {
		return maxNoveltyTemperature
	}
	scaled := float64(novelty) / maxNoveltyLevel * maxNoveltyTemperature * temperatureScale
	return math.Round(scaled) / temperatureScale
}
