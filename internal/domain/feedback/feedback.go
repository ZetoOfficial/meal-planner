package feedback

import (
	"errors"
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
)

type ID int64

type TargetType string

const (
	TargetRecipeTemplate TargetType = "recipe_template"
	TargetProduct        TargetType = "product"
	TargetMeal           TargetType = "meal"
	TargetSlotFill       TargetType = "slot_fill"
)

func (t TargetType) Valid() bool {
	switch t {
	case TargetRecipeTemplate, TargetProduct, TargetMeal, TargetSlotFill:
		return true
	default:
		return false
	}
}

const (
	minRating = -2
	maxRating = 2
	// weightStep converts one rating point into a rotation weight nudge.
	weightStep = 0.1
	// MinWeight keeps a disliked target in rotation: feedback lowers the odds,
	// it never bans (FR-7).
	MinWeight = 0.1
	MaxWeight = 2.0
)

type Event struct {
	ID          ID
	EaterID     eaterdom.ID
	TargetType  TargetType
	TargetID    int64
	Rating      *int
	Comment     *string
	WeightDelta *float64
	CreatedAt   time.Time
}

// Validate rejects targets and ratings the domain does not recognise.
func (e Event) Validate() error {
	if !e.EaterID.Valid() {
		return errors.New("feedback needs an eater")
	}
	if !e.TargetType.Valid() {
		return errors.New("feedback target type is unknown")
	}
	if e.TargetID <= 0 {
		return errors.New("feedback needs a positive target id")
	}
	if e.Rating != nil && (*e.Rating < minRating || *e.Rating > maxRating) {
		return errors.New("feedback rating must be between -2 and 2")
	}
	return nil
}

// WeightDeltaFor turns a rating into a bounded rotation nudge. A comment with
// no rating carries no weight change: the text is stored, not interpreted.
func WeightDeltaFor(rating *int) *float64 {
	if rating == nil || *rating == 0 {
		return nil
	}
	delta := float64(*rating) * weightStep
	return &delta
}
