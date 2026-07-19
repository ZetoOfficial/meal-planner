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
	ID          ID
	Name        string
	Sex         Sex
	Allergens   []Allergen
	Exclusions  []Exclusion
	CurrentGoal *MacroGoal
}

type Allergen string
