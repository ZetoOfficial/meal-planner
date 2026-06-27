package nutrition

// RuleEvalStatus — исход проверки одного quality-правила.
type RuleEvalStatus string

const (
	StatusPass     RuleEvalStatus = "pass"
	StatusSoftFail RuleEvalStatus = "soft_fail"
	StatusWarn     RuleEvalStatus = "warn"
	StatusUnknown  RuleEvalStatus = "unknown"
)

// RuleEvaluationResult — результат правила с explainability. TODO: поля из schema.sql.
type RuleEvaluationResult struct {
	RuleCode string
	Status   RuleEvalStatus
	Message  string
}
