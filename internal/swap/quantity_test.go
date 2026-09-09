package swap

import (
	"errors"
	"testing"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
)

func TestComputeQuantityDoesNotMutateThePlan(t *testing.T) {
	input := quantityInput()

	if _, err := ComputeQuantity(input, quantityGoals(), QuantityRequest{SlotFillID: 1, PlannedInputQtyG: 988}); err != nil {
		t.Fatalf("ComputeQuantity() error = %v", err)
	}

	if got := input.Plan.Recipes[0].Fills[0].PlannedInputQtyG; got != 1200 {
		t.Fatalf("stored quantity = %v g, want the preview to leave the plan untouched", got)
	}
	if got := input.Plan.Meals[0].Components[0].PortionFactor; got != 0.5 {
		t.Fatalf("stored portion = %v, want the preview to leave the allocation untouched", got)
	}
}

func TestComputeQuantityRebalancesAFeasibleChange(t *testing.T) {
	preview, err := ComputeQuantity(
		quantityInput(), quantityGoals(),
		QuantityRequest{SlotFillID: 1, PlannedInputQtyG: 988},
	)
	if err != nil {
		t.Fatalf("ComputeQuantity() error = %v", err)
	}

	if preview.Plan.Recipes[0].Fills[0].PlannedInputQtyG != 988 {
		t.Fatalf("preview quantity = %v, want the locked quantity", preview.Plan.Recipes[0].Fills[0].PlannedInputQtyG)
	}
	if len(preview.PortionChanges) != 1 {
		t.Fatalf("portion changes = %#v, want the affected eater-day rebalanced", preview.PortionChanges)
	}
	change := preview.PortionChanges[0]
	if change.After <= change.Before {
		t.Fatalf("portion change = %#v, want a larger share of the smaller batch", change)
	}
	if len(preview.AffectedMeals) != 1 || preview.AffectedMeals[0] != 21 {
		t.Fatalf("affected meals = %#v, want the meal served from the batch", preview.AffectedMeals)
	}
	if len(preview.ShoppingDiff) != 1 || preview.ShoppingDiff[0].AfterG != 988 {
		t.Fatalf("shopping diff = %#v, want the purchased quantity to follow", preview.ShoppingDiff)
	}
	if preview.Summary == "" {
		t.Fatal("summary is empty, want a readable explanation")
	}
	if len(preview.EaterDays) != 1 {
		t.Fatalf("eater days = %#v, want the re-evaluated snapshot", preview.EaterDays)
	}
}

func TestComputeQuantityKeepsTheDayInsideItsMacroRange(t *testing.T) {
	goals := quantityGoals()
	preview, err := ComputeQuantity(quantityInput(), goals, QuantityRequest{SlotFillID: 1, PlannedInputQtyG: 988})
	if err != nil {
		t.Fatalf("ComputeQuantity() error = %v", err)
	}

	if !goals[0].Range.Contains(preview.Plan.Meals[0].Planned) {
		t.Fatalf("planned macros = %#v, outside the target %#v", preview.Plan.Meals[0].Planned, goals[0].Range)
	}
}

func TestComputeQuantityReportsInfeasibilityWithDiagnostics(t *testing.T) {
	// A pack this small cannot reach the protein floor even at a full portion.
	_, err := ComputeQuantity(quantityInput(), quantityGoals(), QuantityRequest{SlotFillID: 1, PlannedInputQtyG: 50})

	var infeasible *InfeasibleQuantityError
	if !errors.As(err, &infeasible) {
		t.Fatalf("ComputeQuantity() error = %v, want typed infeasibility", err)
	}
	if len(infeasible.Constraints) == 0 {
		t.Fatalf("diagnostics = %#v, want the unsatisfied bound named", infeasible.Constraints)
	}
	if infeasible.Constraints[0].Code != solver.MacroProtein {
		t.Fatalf("constraint = %#v, want the protein bound", infeasible.Constraints[0])
	}
}

func TestComputeQuantityRejectsNonPositiveQuantities(t *testing.T) {
	_, err := ComputeQuantity(quantityInput(), quantityGoals(), QuantityRequest{SlotFillID: 1, PlannedInputQtyG: 0})

	if !errors.Is(err, errNonPositiveQuantity) {
		t.Fatalf("ComputeQuantity() error = %v, want a non-positive quantity error", err)
	}
}

func TestComputeQuantityWarnsAboutAnUnallocatedBatch(t *testing.T) {
	input := quantityInput()
	// A far bigger pack needs a much smaller share, leaving a visible reserve.
	preview, err := ComputeQuantity(input, quantityGoals(), QuantityRequest{SlotFillID: 1, PlannedInputQtyG: 2400})
	if err != nil {
		t.Fatalf("ComputeQuantity() error = %v", err)
	}

	if len(preview.Warnings) == 0 {
		t.Fatalf("warnings = %#v, want the leftover reported", preview.Warnings)
	}
}

// quantityInput holds a 1200 g chicken batch shared at half a portion, so the
// eater's day carries 600 g of chicken.
func quantityInput() Input {
	input := swapInput()
	input.Plan.Recipes[0].Fills[0].PlannedInputQtyG = 1200
	input.Plan.Meals[0].Components[0].PortionFactor = 0.5
	return input
}

func quantityGoals() []GoalTarget {
	return []GoalTarget{{
		EaterID: 1,
		Date:    swapDate(),
		Range: eaterdom.MacroRange{
			ProteinMinG: 130, ProteinMaxG: 145,
			FatMinG: 60, FatMaxG: 90,
			CarbMinG: 0, CarbMaxG: 10,
			KcalMin: 1000, KcalMax: 1400,
		},
	}}
}
