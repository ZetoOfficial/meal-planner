package llm

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"text/template"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

//go:embed prompts/recipe_generation.md
var recipeGenerationPrompt string

const (
	recipeToolName = "emit_recipe_candidates"
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
	Generate(ctx context.Context, request Request) (Response, error)
}

// RecipeGeneratorInput is everything the model may see. Anything not listed
// here is out of v1 scope and must not reach the prompt.
type RecipeGeneratorInput struct {
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

type RecipeGenerator struct {
	client   Completer
	template *template.Template
}

func NewRecipeGenerator(client Completer) (*RecipeGenerator, error) {
	parsed, err := template.New("recipe_generation").Parse(recipeGenerationPrompt)
	if err != nil {
		return nil, fmt.Errorf("parse recipe prompt: %w", err)
	}
	return &RecipeGenerator{client: client, template: parsed}, nil
}

// Generate asks for candidate templates and returns them as draft recipes.
// Drafts are candidates only: the solver still decides whether any of them is
// used, and promotion to permanent happens after the dish is actually cooked.
func (g *RecipeGenerator) Generate(ctx context.Context, input RecipeGeneratorInput) ([]recipedom.Template, error) {
	prompt, err := g.Prompt(input)
	if err != nil {
		return nil, err
	}

	response, err := g.client.Generate(ctx, Request{
		System:      "You propose candidate recipe templates and answer only through the provided tool.",
		Prompt:      prompt,
		ToolName:    recipeToolName,
		Schema:      json.RawMessage(recipeCandidateSchema),
		Temperature: NoveltyTemperature(input.Novelty),
		PlanID:      input.PlanID,
	})
	if err != nil {
		return nil, err
	}
	return parseRecipeCandidates(response.Content, input)
}

// Prompt renders the versioned prompt file for the supplied constraints.
func (g *RecipeGenerator) Prompt(input RecipeGeneratorInput) (string, error) {
	if input.Count <= 0 {
		input.Count = defaultCandidateCount
	}
	var rendered bytes.Buffer
	if err := g.template.Execute(&rendered, input); err != nil {
		return "", fmt.Errorf("render recipe prompt: %w", err)
	}
	return rendered.String(), nil
}

// NoveltyTemperature maps the novelty dial onto provider temperature: level 0
// is deterministic, level 3 is "surprise me".
func NoveltyTemperature(novelty int) float64 {
	if novelty <= 0 {
		return 0
	}
	if novelty >= maxNoveltyLevel {
		return maxNoveltyTemperature
	}
	scaled := float64(novelty) / maxNoveltyLevel * maxNoveltyTemperature * temperatureScale
	return math.Round(scaled) / temperatureScale
}

const recipeCandidateSchema = `{
  "type": "object",
  "properties": {
    "recipes": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "component_kind": {"type": "string", "enum": ["protein", "carb_side", "vegetable_side", "sauce", "mixed_main"]},
          "allowed_meal_types": {"type": "array", "items": {"type": "string", "enum": ["breakfast", "lunch", "dinner", "snack"]}},
          "base_servings": {"type": "integer", "minimum": 1},
          "expected_output_g": {"type": "number"},
          "cuisine": {"type": "string"},
          "cooking_method": {"type": "string"},
          "storage": {"type": "string", "enum": ["keeps_3d", "keeps_3d_reheat", "assemble_fresh", "eat_same_day"]},
          "storage_notes": {"type": "string"},
          "equipment": {"type": "array", "items": {"type": "string"}},
          "instructions": {"type": "string"},
          "slots": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "role_tag_code": {"type": "string"},
                "label": {"type": "string"},
                "qty_g": {"type": "number"},
                "prepared_qty_factor": {"type": "number"},
                "optional": {"type": "boolean"}
              },
              "required": ["role_tag_code", "label", "qty_g"]
            }
          }
        },
        "required": ["name", "component_kind", "allowed_meal_types", "base_servings", "storage", "slots"]
      }
    }
  },
  "required": ["recipes"]
}`

type recipeCandidatePayload struct {
	Recipes []recipeCandidate `json:"recipes"`
}

type recipeCandidate struct {
	Name             string                `json:"name"`
	ComponentKind    string                `json:"component_kind"`
	AllowedMealTypes []string              `json:"allowed_meal_types"`
	BaseServings     int                   `json:"base_servings"`
	ExpectedOutputG  *float64              `json:"expected_output_g"`
	Cuisine          *string               `json:"cuisine"`
	CookingMethod    *string               `json:"cooking_method"`
	Storage          string                `json:"storage"`
	StorageNotes     *string               `json:"storage_notes"`
	Equipment        []string              `json:"equipment"`
	Instructions     *string               `json:"instructions"`
	Slots            []recipeCandidateSlot `json:"slots"`
}

type recipeCandidateSlot struct {
	RoleTagCode       string   `json:"role_tag_code"`
	Label             string   `json:"label"`
	QtyG              float64  `json:"qty_g"`
	PreparedQtyFactor *float64 `json:"prepared_qty_factor"`
	Optional          bool     `json:"optional"`
}

// parseRecipeCandidates turns structured output into draft templates and
// rejects anything the household cannot actually cook.
func parseRecipeCandidates(content string, input RecipeGeneratorInput) ([]recipedom.Template, error) {
	var payload recipeCandidatePayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nil, apperr.Validation("llm returned malformed recipe candidates", err.Error())
	}
	if len(payload.Recipes) == 0 {
		return nil, apperr.Validation("llm returned no recipe candidates")
	}

	roles := codeSet(roleCodes(input.RoleTags))
	equipment := codeSet(input.Equipment)
	mealTypes := codeSet(mealTypeCodes(input.MealTypes))

	templates := make([]recipedom.Template, 0, len(payload.Recipes))
	for i := range payload.Recipes {
		candidate, err := payload.Recipes[i].toTemplate(roles, equipment, mealTypes)
		if err != nil {
			return nil, err
		}
		templates = append(templates, candidate)
	}
	return templates, nil
}

func (c recipeCandidate) toTemplate(roles, equipment, mealTypes map[string]struct{}) (recipedom.Template, error) {
	name := strings.TrimSpace(c.Name)
	if name == "" {
		return recipedom.Template{}, apperr.Validation("llm recipe candidate has no name")
	}
	kind := recipedom.ComponentKind(c.ComponentKind)
	storage := recipedom.StorageProfile(c.Storage)
	if !kind.Valid() || !storage.Valid() || c.BaseServings <= 0 {
		return recipedom.Template{}, apperr.Validation("llm recipe candidate has an invalid batch contract", name)
	}
	if c.ExpectedOutputG != nil && !positive(*c.ExpectedOutputG) {
		return recipedom.Template{}, apperr.Validation("llm recipe candidate has a non-positive expected output", name)
	}

	allowed, err := candidateMealTypes(name, c.AllowedMealTypes, mealTypes)
	if err != nil {
		return recipedom.Template{}, err
	}
	for _, item := range c.Equipment {
		if _, ok := equipment[item]; !ok {
			return recipedom.Template{}, apperr.Validation("llm recipe candidate needs unavailable equipment", item)
		}
	}
	slots, err := candidateSlots(name, c.Slots, roles)
	if err != nil {
		return recipedom.Template{}, err
	}

	return recipedom.Template{
		Name:             name,
		Status:           recipedom.StatusDraft,
		Source:           recipedom.SourceLLM,
		ComponentKind:    kind,
		AllowedMealTypes: allowed,
		BaseServings:     c.BaseServings,
		ExpectedOutputG:  c.ExpectedOutputG,
		Cuisine:          c.Cuisine,
		CookingMethod:    c.CookingMethod,
		Storage:          storage,
		StorageNotes:     c.StorageNotes,
		Equipment:        append([]string(nil), c.Equipment...),
		Instructions:     c.Instructions,
		Slots:            slots,
	}, nil
}

func candidateMealTypes(name string, values []string, allowed map[string]struct{}) ([]mealdom.Type, error) {
	if len(values) == 0 {
		return nil, apperr.Validation("llm recipe candidate allows no meal type", name)
	}
	result := make([]mealdom.Type, 0, len(values))
	for _, value := range values {
		mealType := mealdom.Type(value)
		if !mealType.Valid() {
			return nil, apperr.Validation("llm recipe candidate uses an unknown meal type", value)
		}
		if _, ok := allowed[value]; !ok {
			return nil, apperr.Validation("llm recipe candidate uses a meal type outside the request", value)
		}
		result = append(result, mealType)
	}
	return result, nil
}

func candidateSlots(name string, slots []recipeCandidateSlot, roles map[string]struct{}) ([]recipedom.Slot, error) {
	if len(slots) == 0 {
		return nil, apperr.Validation("llm recipe candidate has no slots", name)
	}
	result := make([]recipedom.Slot, 0, len(slots))
	for position, slot := range slots {
		if _, ok := roles[slot.RoleTagCode]; !ok {
			return nil, apperr.Validation("llm recipe candidate uses an unknown role tag", slot.RoleTagCode)
		}
		if !positive(slot.QtyG) {
			return nil, apperr.Validation("llm recipe slot quantity must be positive", slot.RoleTagCode)
		}
		if slot.PreparedQtyFactor != nil && !positive(*slot.PreparedQtyFactor) {
			return nil, apperr.Validation("llm recipe slot prepared factor must be positive", slot.RoleTagCode)
		}
		result = append(result, recipedom.Slot{
			RoleTagCode:       slot.RoleTagCode,
			Label:             strings.TrimSpace(slot.Label),
			QtyG:              slot.QtyG,
			PreparedQtyFactor: slot.PreparedQtyFactor,
			Position:          position,
			Optional:          slot.Optional,
		})
	}
	return result, nil
}

func roleCodes(roles []catalogdom.RoleTag) []string {
	codes := make([]string, 0, len(roles))
	for _, role := range roles {
		codes = append(codes, role.Code)
	}
	return codes
}

func mealTypeCodes(types []mealdom.Type) []string {
	codes := make([]string, 0, len(types))
	for _, mealType := range types {
		codes = append(codes, string(mealType))
	}
	return codes
}

func codeSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func positive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}
