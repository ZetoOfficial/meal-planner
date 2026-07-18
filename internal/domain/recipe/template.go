package recipe

import "time"

type ID int64

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPermanent Status = "permanent"
	StatusRejected  Status = "rejected"
)

type Source string

const (
	SourceLLM    Source = "llm"
	SourceManual Source = "manual"
)

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

type TemplateDraft struct {
	Name  string
	Slots []Slot
}
