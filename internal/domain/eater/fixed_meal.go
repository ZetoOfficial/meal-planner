package eater

type FixedMealID int64

type MealType string

const (
	MealTypeBreakfast MealType = "breakfast"
	MealTypeLunch     MealType = "lunch"
	MealTypeDinner    MealType = "dinner"
	MealTypeSnack     MealType = "snack"
)

func (t MealType) Valid() bool {
	return t == MealTypeBreakfast || t == MealTypeLunch || t == MealTypeDinner || t == MealTypeSnack
}

type Macros struct {
	ProteinG float64
	FatG     float64
	CarbG    float64
	Kcal     int
}

type FixedMealQuality struct {
	SaturatedFatG    *float64
	TransFatG        *float64
	FiberG           *float64
	SodiumMg         *float64
	AddedSugarsG     *float64
	FreeSugarsG      *float64
	ProcessedMeatG   *float64
	RedMeatG         *float64
	WholeGrainGEquiv *float64
	FruitVegG        *float64

	IncludeInAnalytics bool
	IncludeInWarnings  bool
}

type FixedMeal struct {
	ID       FixedMealID
	EaterID  ID
	MealType MealType
	Label    string
	Macros   Macros
	Quality  FixedMealQuality
	Active   bool
}
