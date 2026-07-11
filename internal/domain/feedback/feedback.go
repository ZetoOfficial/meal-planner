package feedback

type TargetType string

type Event struct {
	TargetType TargetType
	TargetID   int64
}
