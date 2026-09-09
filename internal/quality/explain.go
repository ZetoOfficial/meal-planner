package quality

import (
	"fmt"

	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

func explainOutcome(code string, status nutrition.RuleEvalStatus, actual, target *float64, unit string) string {
	switch status {
	case nutrition.StatusPass:
		return fmt.Sprintf("%s satisfied: %s %s", code, formatActual(actual), unit)
	case nutrition.StatusWarn:
		return fmt.Sprintf("%s warning: %s %s (target %s %s)", code, formatActual(actual), unit, formatTarget(target), unit)
	case nutrition.StatusSoftFail:
		return fmt.Sprintf("%s soft fail: %s %s (target %s %s)", code, formatActual(actual), unit, formatTarget(target), unit)
	case nutrition.StatusUnknown:
		return fmt.Sprintf("%s unknown: insufficient data (coverage %s)", code, formatActual(actual))
	default:
		return fmt.Sprintf("%s: status %s", code, status)
	}
}

func formatActual(v *float64) string {
	if v == nil {
		return "unknown"
	}
	return fmt.Sprintf("%.2f", *v)
}

func formatTarget(v *float64) string {
	if v == nil {
		return "unknown"
	}
	return fmt.Sprintf("%.2f", *v)
}

// Result narrows an outcome to the persisted rule verdict, so the stored
// explanation is exactly the one the ranking produced.
func (o RuleOutcome) Result() nutrition.RuleEvaluationResult {
	return nutrition.RuleEvaluationResult{
		RuleCode: o.Code,
		Status:   o.Status,
		Actual:   o.Actual,
		Target:   o.Target,
		Unit:     o.Unit,
		Coverage: o.Coverage,
		Message:  o.Message,
	}
}
