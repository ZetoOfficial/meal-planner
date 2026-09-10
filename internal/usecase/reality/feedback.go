package reality

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	feedbackdom "github.com/ZetoOfficial/meal-planner/internal/domain/feedback"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

// CreateFeedback stores the owner's verdict and nudges the rotation weight of a
// recipe. Feedback moves probability, it never bans: a disliked recipe keeps a
// floor weight and stays in the library.
//
// Only recipe templates carry a rotation weight in the schema, so feedback on a
// product, meal or slot fill is recorded without moving any weight.
func (s *Service) CreateFeedback(ctx context.Context, event feedbackdom.Event) (feedbackdom.Event, error) {
	if err := event.Validate(); err != nil {
		return feedbackdom.Event{}, apperr.Validation("feedback is not valid", err.Error())
	}
	event.WeightDelta = feedbackdom.WeightDeltaFor(event.Rating)

	var stored feedbackdom.Event
	err := s.transactions.WithTx(ctx, func(ctx context.Context) error {
		created, err := s.feedback.Create(ctx, event)
		if err != nil {
			return err
		}
		stored = created

		if event.TargetType != feedbackdom.TargetRecipeTemplate || event.WeightDelta == nil {
			return nil
		}
		return s.feedback.AdjustRecipeWeight(ctx, recipedom.ID(event.TargetID), *event.WeightDelta)
	})
	if err != nil {
		return feedbackdom.Event{}, err
	}
	return stored, nil
}
