package meal

type Type string

const (
	TypeBreakfast Type = "breakfast"
	TypeLunch     Type = "lunch"
	TypeDinner    Type = "dinner"
	TypeSnack     Type = "snack"
)

func (t Type) Valid() bool {
	return t == TypeBreakfast || t == TypeLunch || t == TypeDinner || t == TypeSnack
}
