package eater

type ExclusionID int64

func (id ExclusionID) Valid() bool {
	return id > 0
}

type ExclusionTarget string

const (
	ExclusionTargetFoodGroup ExclusionTarget = "food_group"
	ExclusionTargetRoleTag   ExclusionTarget = "role_tag"
	ExclusionTargetProduct   ExclusionTarget = "product"
)

type ExclusionSeverity string

const (
	ExclusionSeverityHard ExclusionSeverity = "hard"
	ExclusionSeveritySoft ExclusionSeverity = "soft"
)

type Exclusion struct {
	ID          ExclusionID
	EaterID     ID
	TargetKind  ExclusionTarget
	TargetValue string
	Severity    ExclusionSeverity
	Note        *string
}
