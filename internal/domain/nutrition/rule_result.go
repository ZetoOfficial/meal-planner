package nutrition

type RuleEvalStatus string

const (
	StatusPass     RuleEvalStatus = "pass"
	StatusSoftFail RuleEvalStatus = "soft_fail"
	StatusWarn     RuleEvalStatus = "warn"
	StatusUnknown  RuleEvalStatus = "unknown"
)

// NeedsAttention reports the verdicts counted as plan warnings.
func (s RuleEvalStatus) NeedsAttention() bool {
	return s == StatusSoftFail || s == StatusWarn
}

func (s RuleEvalStatus) Valid() bool {
	switch s {
	case StatusPass, StatusSoftFail, StatusWarn, StatusUnknown:
		return true
	default:
		return false
	}
}

// RuleEvaluationResult is one persisted rule verdict: what was compared, how
// confident the comparison was, and how to explain it to the owner.
type RuleEvaluationResult struct {
	RuleCode string
	Status   RuleEvalStatus
	Actual   *float64
	Target   *float64
	Unit     string
	Coverage float64
	Message  string
}
