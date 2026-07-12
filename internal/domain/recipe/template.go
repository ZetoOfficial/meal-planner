package recipe

type ID int64

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPermanent Status = "permanent"
	StatusRejected  Status = "rejected"
)

type Template struct {
	ID     ID
	Name   string
	Status Status
	Slots  []Slot
}

type TemplateDraft struct {
	Name  string
	Slots []Slot
}
