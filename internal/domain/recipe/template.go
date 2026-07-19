package recipe

import "time"

type ID int64

func (id ID) Valid() bool {
	return id > 0
}

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPermanent Status = "permanent"
	StatusRejected  Status = "rejected"
)

func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusPermanent, StatusRejected:
		return true
	default:
		return false
	}
}

type Source string

const (
	SourceLLM    Source = "llm"
	SourceManual Source = "manual"
)

func (s Source) Valid() bool {
	switch s {
	case SourceLLM, SourceManual:
		return true
	default:
		return false
	}
}

type Template struct {
	ID            ID
	Name          string
	Status        Status
	Source        Source
	Cuisine       *string
	CookingMethod *string
	Storage       StorageProfile
	StorageNotes  *string
	Equipment     []string
	Instructions  *string
	Weight        float64
	CookedCount   int
	PromotedAt    *time.Time
	Slots         []Slot
}

func (t Template) RoleCodes() []string {
	result := make([]string, 0, len(t.Slots))
	seen := make(map[string]struct{}, len(t.Slots))
	for _, slot := range t.Slots {
		if _, exists := seen[slot.RoleTagCode]; exists {
			continue
		}
		seen[slot.RoleTagCode] = struct{}{}
		result = append(result, slot.RoleTagCode)
	}
	return result
}

type TemplateDraft struct {
	Name  string
	Slots []Slot
}
