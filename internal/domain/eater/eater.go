// Package eater — профили едоков (макро-цели, аллергены, fixed meals, quality-профиль).
// Важно: макро-цели и quality-правила НЕ смешиваются (ARCHITECTURE §5.3).
package eater

// ID — идентификатор едока.
type ID int64

// Sex используется только для дефолта клетчатки (25 ж / 35 м).
type Sex string

const (
	SexMale        Sex = "male"
	SexFemale      Sex = "female"
	SexUnspecified Sex = "unspecified"
)

// Eater — профиль едока. TODO: связать с MacroGoal/FixedMeal/QualityProfile.
type Eater struct {
	ID   ID
	Name string
	Sex  Sex
}
