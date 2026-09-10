package solver

import (
	"testing"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

func TestDiagnosticsExplainMissingRole(t *testing.T) {
	input := rankingInput(rankingProduct(1, withFiber(40), withRole("main:protein")))
	input.Recipes[0].Slots[0].RoleTagCode = "main:mixed"

	diagnostics := failureDiagnostics(t, input)

	assertDiagnostic(t, diagnostics, DiagnosticMissingRole, "main:mixed")
}

func TestDiagnosticsExplainAllergenConflict(t *testing.T) {
	input := rankingInput(rankingProduct(1, withFiber(40)))
	input.Products[0].Allergens = []catalogdom.Allergen{"milk"}
	input.Eaters[0].Eater.Allergens = []eaterdom.Allergen{"milk"}

	diagnostics := failureDiagnostics(t, input)

	assertDiagnostic(t, diagnostics, DiagnosticAllergenConflict, "milk")
}

func TestDiagnosticsExplainHardExclusionConflict(t *testing.T) {
	input := rankingInput(rankingProduct(1, withFiber(40)))
	input.Eaters[0].Eater.Exclusions = []eaterdom.Exclusion{{
		TargetKind:  eaterdom.ExclusionTargetRoleTag,
		TargetValue: "main:mixed",
		Severity:    eaterdom.ExclusionSeverityHard,
	}}

	diagnostics := failureDiagnostics(t, input)

	assertDiagnostic(t, diagnostics, DiagnosticExclusionConflict, "main:mixed")
}

func TestDiagnosticsExplainEquipmentConflict(t *testing.T) {
	input := rankingInput(rankingProduct(1, withFiber(40)))
	input.Recipes[0].Equipment = []string{"sous_vide"}

	diagnostics := failureDiagnostics(t, input)

	assertDiagnostic(t, diagnostics, DiagnosticEquipmentConflict, "sous_vide")
}

func TestDiagnosticsExplainStorageConflict(t *testing.T) {
	input := rankingInput(rankingProduct(1, withFiber(40)))
	input.Days = 2
	input.Recipes[0].Storage = recipedom.StorageEatSameDay
	input.Eaters[0].Days = append(input.Eaters[0].Days, input.Eaters[0].Days[0])
	input.Eaters[0].Days[1].Date = input.StartDate.AddDate(0, 0, 1)

	diagnostics := failureDiagnostics(t, input)

	assertDiagnostic(t, diagnostics, DiagnosticStorageConflict, string(recipedom.StorageEatSameDay))
}

func TestDiagnosticsExplainMacroInfeasibilityWithoutRejections(t *testing.T) {
	input := rankingInput(rankingProduct(1, withFiber(40)))
	input.Eaters[0].Days[0].Goal.Range.ProteinMinG = 500
	input.Eaters[0].Days[0].Goal.Range.ProteinMaxG = 600

	diagnostics := failureDiagnostics(t, input)

	assertDiagnostic(t, diagnostics, DiagnosticMacroInfeasible, "")
}

func TestDiagnosticsExplainSearchLimit(t *testing.T) {
	planner := NewWithConfig(Config{ServingMultipliers: []float64{0.5, 1}, SearchBudget: 1})

	result, err := planner.BuildPlan(t.Context(), rankingInput(rankingProduct(1, withFiber(40))))
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if result.Failure == nil {
		t.Fatalf("BuildPlan() = %#v, want a search limit failure", result.Plan)
	}

	assertDiagnostic(t, result.Failure.Diagnostics(), DiagnosticSearchLimit, "")
}

func TestDiagnosticsAreDeduplicatedAndReadable(t *testing.T) {
	input := rankingInput(
		rankingProduct(1, withFiber(40)),
		rankingProduct(2, withFiber(40)),
	)
	input.Products[0].Allergens = []catalogdom.Allergen{"milk"}
	input.Products[1].Allergens = []catalogdom.Allergen{"milk"}
	input.Eaters[0].Eater.Allergens = []eaterdom.Allergen{"milk"}

	diagnostics := failureDiagnostics(t, input)

	allergen := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != DiagnosticAllergenConflict {
			continue
		}
		allergen++
		if diagnostic.Message == "" {
			t.Fatalf("diagnostic %#v has no readable message", diagnostic)
		}
	}
	if allergen != 1 {
		t.Fatalf("allergen diagnostics = %d, want one deduplicated entry for the shared allergen", allergen)
	}
}

func failureDiagnostics(t *testing.T, input Input) []Diagnostic {
	t.Helper()
	result, err := New().BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if result.Failure == nil {
		t.Fatalf("BuildPlan() = %#v, want an impossible plan", result.Plan)
	}
	return result.Failure.Diagnostics()
}

func assertDiagnostic(t *testing.T, diagnostics []Diagnostic, code DiagnosticCode, detail string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != code || diagnostic.Detail != detail {
			continue
		}
		if diagnostic.Message == "" {
			t.Fatalf("diagnostic %#v has no readable message", diagnostic)
		}
		return
	}
	t.Fatalf("diagnostics = %#v, want code %q with detail %q", diagnostics, code, detail)
}

func TestFailureNamesTheUnreachableMacro(t *testing.T) {
	input := rankingInput(rankingProduct(1, withFiber(40)))
	input.Eaters[0].Days[0].Goal.Range.ProteinMinG = 500
	input.Eaters[0].Days[0].Goal.Range.ProteinMaxG = 600

	result, err := New().BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if result.Failure == nil {
		t.Fatalf("BuildPlan() = %#v, want an impossible plan", result.Plan)
	}

	constraint, found := macroConstraint(result.Failure.MacroConstraints, MacroProtein)
	if !found {
		t.Fatalf("macro constraints = %#v, want the unreachable protein bound", result.Failure.MacroConstraints)
	}
	if constraint.TargetMin != 500 || constraint.ReachableMax != 60 {
		t.Fatalf("protein constraint = %#v, want target 500 g against 60 g reachable", constraint)
	}
}

func TestFailureReportsNoMacroConstraintWhenTargetsAreReachable(t *testing.T) {
	// Candidates cover the target range, but no single combination hits it.
	input := rankingInput(rankingProduct(1, withFiber(40)))
	input.Eaters[0].Days[0].Goal.Range.ProteinMinG = 31
	input.Eaters[0].Days[0].Goal.Range.ProteinMaxG = 32

	result, err := New().BuildPlan(t.Context(), input)
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if result.Failure == nil {
		t.Fatalf("BuildPlan() = %#v, want an impossible plan", result.Plan)
	}
	if len(result.Failure.MacroConstraints) != 0 {
		t.Fatalf("macro constraints = %#v, want none when every bound is individually reachable", result.Failure.MacroConstraints)
	}
}

func macroConstraint(constraints []MacroConstraint, code MacroCode) (MacroConstraint, bool) {
	for i := range constraints {
		if constraints[i].Code == code {
			return constraints[i], true
		}
	}
	return MacroConstraint{}, false
}
