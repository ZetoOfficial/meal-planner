package llm

import (
	"context"
	"errors"
	"strings"
	"testing"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

func TestPromptCarriesOnlyTheSuppliedConstraints(t *testing.T) {
	generator := newGenerator(t, nil)

	prompt, err := generator.Prompt(generatorInput())
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}

	for _, expected := range []string{
		"Chicken breast",
		"protein:lean",
		"veg:green",
		"milk",
		"food group: confectionery",
		"oven",
		"lunch",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt does not mention %q:\n%s", expected, prompt)
		}
	}
	for _, forbidden := range []string{"barcode", "photo", "inventory", "Beef mince"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("prompt leaks out-of-scope input %q", forbidden)
		}
	}
}

func TestNoveltyMapsToProviderTemperature(t *testing.T) {
	cases := map[int]float64{0: 0, 1: 0.27, 2: 0.53, 3: 0.8, 9: 0.8}
	for novelty, want := range cases {
		if got := NoveltyTemperature(novelty); got != want {
			t.Fatalf("NoveltyTemperature(%d) = %v, want %v", novelty, got, want)
		}
	}
}

func TestGenerateParsesCandidatesAsDrafts(t *testing.T) {
	completer := &fakeCompleter{content: `{"recipes":[{
		"name":" Lemon chicken ",
		"component_kind":"protein",
		"allowed_meal_types":["lunch"],
		"base_servings":4,
		"expected_output_g":720,
		"storage":"keeps_3d",
		"equipment":["oven"],
		"slots":[{"role_tag_code":"protein:lean","label":"Chicken","qty_g":960,"prepared_qty_factor":0.75}]
	}]}`}
	generator := newGenerator(t, completer)

	templates, err := generator.Generate(t.Context(), generatorInput())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(templates) != 1 {
		t.Fatalf("templates = %d, want one candidate", len(templates))
	}
	candidate := templates[0]
	if candidate.Status != recipedom.StatusDraft || candidate.Source != recipedom.SourceLLM {
		t.Fatalf("candidate = %#v, want an llm draft rather than a usable plan", candidate)
	}
	if candidate.Name != "Lemon chicken" || candidate.BaseServings != 4 {
		t.Fatalf("candidate = %#v, want the parsed batch contract", candidate)
	}
	if len(candidate.Slots) != 1 || candidate.Slots[0].QtyG != 960 || candidate.Slots[0].Position != 0 {
		t.Fatalf("candidate slots = %#v, want the parsed slot", candidate.Slots)
	}
	if completer.request.Temperature != 0.53 || completer.request.ToolName != recipeToolName {
		t.Fatalf("provider request = %#v, want the novelty temperature and the recipe tool", completer.request)
	}
}

func TestGenerateRejectsUnknownRoleTags(t *testing.T) {
	completer := &fakeCompleter{content: candidateWith(`"slots":[{"role_tag_code":"protein:unicorn","label":"X","qty_g":100}]`)}

	assertValidationFailure(t, newGenerator(t, completer))
}

func TestGenerateRejectsUnavailableEquipment(t *testing.T) {
	completer := &fakeCompleter{content: candidateWith(`"equipment":["sous_vide"],"slots":[{"role_tag_code":"protein:lean","label":"X","qty_g":100}]`)}

	assertValidationFailure(t, newGenerator(t, completer))
}

func TestGenerateRejectsMealTypesOutsideTheRequest(t *testing.T) {
	completer := &fakeCompleter{content: `{"recipes":[{
		"name":"Breakfast bowl","component_kind":"protein","allowed_meal_types":["breakfast"],
		"base_servings":2,"storage":"keeps_3d",
		"slots":[{"role_tag_code":"protein:lean","label":"X","qty_g":100}]
	}]}`}

	assertValidationFailure(t, newGenerator(t, completer))
}

func TestGenerateRejectsNonPositiveQuantities(t *testing.T) {
	completer := &fakeCompleter{content: candidateWith(`"slots":[{"role_tag_code":"protein:lean","label":"X","qty_g":0}]`)}

	assertValidationFailure(t, newGenerator(t, completer))
}

func TestGenerateRejectsMalformedStructuredOutput(t *testing.T) {
	completer := &fakeCompleter{content: `{"recipes": "not an array"}`}

	assertValidationFailure(t, newGenerator(t, completer))
}

func TestGenerateRejectsEmptyCandidateList(t *testing.T) {
	completer := &fakeCompleter{content: `{"recipes":[]}`}

	assertValidationFailure(t, newGenerator(t, completer))
}

func TestGeneratePropagatesProviderFailure(t *testing.T) {
	providerErr := apperr.Unavailable("provider down", errors.New("529"))
	generator := newGenerator(t, &fakeCompleter{err: providerErr})

	_, err := generator.Generate(t.Context(), generatorInput())

	if !errors.Is(err, providerErr) {
		t.Fatalf("Generate() error = %v, want the provider failure surfaced", err)
	}
}

func assertValidationFailure(t *testing.T, generator *RecipeGenerator) {
	t.Helper()
	_, err := generator.Generate(t.Context(), generatorInput())

	var appError *apperr.Error
	if !errors.As(err, &appError) || appError.Code != apperr.CodeValidation {
		t.Fatalf("Generate() error = %v, want a validation error", err)
	}
}

func candidateWith(extra string) string {
	return `{"recipes":[{
		"name":"Candidate","component_kind":"protein","allowed_meal_types":["lunch"],
		"base_servings":2,"storage":"keeps_3d",` + extra + `}]}`
}

func newGenerator(t *testing.T, completer Completer) *RecipeGenerator {
	t.Helper()
	generator, err := NewRecipeGenerator(completer)
	if err != nil {
		t.Fatalf("NewRecipeGenerator() error = %v", err)
	}
	return generator
}

func generatorInput() RecipeGeneratorInput {
	return RecipeGeneratorInput{
		Days:       3,
		Count:      2,
		Novelty:    2,
		MealTypes:  []mealdom.Type{mealdom.TypeLunch},
		Equipment:  []string{"oven", "stove"},
		Allergens:  []string{"milk"},
		Exclusions: []string{"food group: confectionery"},
		RoleTags: []catalogdom.RoleTag{
			{Code: "protein:lean", Kind: catalogdom.RoleKindProtein, Label: "Lean protein"},
			{Code: "veg:green", Kind: catalogdom.RoleKindVeg, Label: "Green vegetables"},
		},
		Products: []catalogdom.Product{{
			ID: 1, Brand: "Acme", Name: "Chicken breast",
			ProteinPer100G: 23, FatPer100G: 2, CarbPer100G: 0, KcalPer100G: 110,
			Roles: []catalogdom.RoleTag{{Code: "protein:lean"}},
		}},
	}
}

type fakeCompleter struct {
	content string
	err     error
	request Request
}

func (c *fakeCompleter) Generate(_ context.Context, request Request) (Response, error) {
	c.request = request
	if c.err != nil {
		return Response{}, c.err
	}
	return Response{Content: c.content}, nil
}
