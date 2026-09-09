package eater

type ID int64

func (id ID) Valid() bool {
	return id > 0
}

type Sex string

const (
	SexMale        Sex = "male"
	SexFemale      Sex = "female"
	SexUnspecified Sex = "unspecified"
)

func (s Sex) Valid() bool {
	return s == SexMale || s == SexFemale || s == SexUnspecified
}

type Eater struct {
	ID         ID
	Name       string
	Sex        Sex
	Allergens  []Allergen
	Exclusions []Exclusion
	// Goals is the stored history, future versions included. Which one is
	// current depends on today's date, so only a use case with a clock may
	// decide it; CurrentGoal is that decision, not stored state.
	Goals       []MacroGoal
	CurrentGoal *MacroGoal
}

type Allergen string
