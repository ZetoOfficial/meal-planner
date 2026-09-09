package solver

import (
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

// MacroCode names the macro a bound belongs to, matching the constraint codes
// the API reports in solver diagnostics.
type MacroCode string

const (
	MacroProtein MacroCode = "protein_g"
	MacroFat     MacroCode = "fat_g"
	MacroCarb    MacroCode = "carb_g"
	MacroKcal    MacroCode = "kcal"
)

// MacroConstraint records a target bound that the available candidates cannot
// reach even in the best case, together with the range they can reach.
type MacroConstraint struct {
	Code         MacroCode
	TargetMin    float64
	TargetMax    float64
	ReachableMin float64
	ReachableMax float64
}

// UnreachableMacros compares a target against the widest achievable range and
// reports the bounds that no combination can reach.
//
// unreachableMacros compares the target against the widest achievable range:
// the fixed meals alone as the floor and the fixed meals plus every candidate
// at its largest serving as the ceiling. Bounds outside that range are provably
// impossible, unlike a target that simply has no exact combination.
func UnreachableMacros(target eaterdom.MacroRange, floor, ceiling nutrition.Macros) []MacroConstraint {
	return unreachableMacros(target, floor, ceiling)
}

func unreachableMacros(target eaterdom.MacroRange, floor, ceiling nutrition.Macros) []MacroConstraint {
	bounds := []MacroConstraint{
		{Code: MacroProtein, TargetMin: target.ProteinMinG, TargetMax: target.ProteinMaxG, ReachableMin: floor.ProteinG, ReachableMax: ceiling.ProteinG},
		{Code: MacroFat, TargetMin: target.FatMinG, TargetMax: target.FatMaxG, ReachableMin: floor.FatG, ReachableMax: ceiling.FatG},
		{Code: MacroCarb, TargetMin: target.CarbMinG, TargetMax: target.CarbMaxG, ReachableMin: floor.CarbG, ReachableMax: ceiling.CarbG},
		{Code: MacroKcal, TargetMin: float64(target.KcalMin), TargetMax: float64(target.KcalMax), ReachableMin: float64(floor.Kcal), ReachableMax: float64(ceiling.Kcal)},
	}

	result := make([]MacroConstraint, 0, len(bounds))
	for _, bound := range bounds {
		if bound.ReachableMax < bound.TargetMin || bound.ReachableMin > bound.TargetMax {
			result = append(result, bound)
		}
	}
	return result
}
