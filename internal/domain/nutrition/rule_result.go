package nutrition

type RuleEvalStatus string

const (
	StatusPass     RuleEvalStatus = "pass"
	StatusSoftFail RuleEvalStatus = "soft_fail"
	StatusWarn     RuleEvalStatus = "warn"
	StatusUnknown  RuleEvalStatus = "unknown"
)

type RuleEvaluationResult struct {
	RuleCode string
	Status   RuleEvalStatus
	Message  string
}
