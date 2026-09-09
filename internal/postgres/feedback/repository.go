// Package feedback stores rating events and nudges rotation weights.
package feedback

import (
	"context"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	feedbackdom "github.com/ZetoOfficial/meal-planner/internal/domain/feedback"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/dberror"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/pgvalue"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

type Repository struct {
	store *postgres.Store
}

func New(store *postgres.Store) *Repository {
	return &Repository{store: store}
}

func (r *Repository) Create(ctx context.Context, event feedbackdom.Event) (feedbackdom.Event, error) {
	var rating *int16
	if event.Rating != nil {
		if *event.Rating < math.MinInt16 || *event.Rating > math.MaxInt16 {
			return feedbackdom.Event{}, apperr.Validation("feedback rating is out of range", "rating")
		}
		value := int16(*event.Rating)
		rating = &value
	}
	row, err := r.store.Queries(ctx).CreateFeedbackEvent(ctx, sqlc.CreateFeedbackEventParams{
		EaterID:     int64(event.EaterID),
		TargetType:  sqlc.FeedbackTarget(event.TargetType),
		TargetID:    event.TargetID,
		Rating:      rating,
		Comment:     event.Comment,
		WeightDelta: pgvalue.OptionalNumeric(event.WeightDelta),
	})
	if err != nil {
		return feedbackdom.Event{}, dberror.Map(err, "feedback")
	}
	return eventFromRow(row)
}

// AdjustRecipeWeight nudges a template's rotation weight inside bounds. The
// floor keeps a disliked recipe in rotation: feedback lowers the odds, never
// removes the option.
func (r *Repository) AdjustRecipeWeight(
	ctx context.Context,
	recipeID recipedom.ID,
	delta float64,
) error {
	_, err := r.store.Queries(ctx).AdjustRecipeWeight(ctx, sqlc.AdjustRecipeWeightParams{
		ID:      int64(recipeID),
		Delta:   pgvalue.Numeric(delta),
		Minimum: pgvalue.Numeric(feedbackdom.MinWeight),
		Maximum: pgvalue.Numeric(feedbackdom.MaxWeight),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("recipe")
	}
	if err != nil {
		return dberror.Map(err, "recipe")
	}
	return nil
}

func eventFromRow(row sqlc.FeedbackEvent) (feedbackdom.Event, error) {
	weightDelta, err := pgvalue.OptionalFloat64(row.WeightDelta)
	if err != nil {
		return feedbackdom.Event{}, err
	}
	var rating *int
	if row.Rating != nil {
		value := int(*row.Rating)
		rating = &value
	}
	return feedbackdom.Event{
		ID:          feedbackdom.ID(row.ID),
		EaterID:     eaterdom.ID(row.EaterID),
		TargetType:  feedbackdom.TargetType(row.TargetType),
		TargetID:    row.TargetID,
		Rating:      rating,
		Comment:     row.Comment,
		WeightDelta: weightDelta,
		CreatedAt:   row.CreatedAt.Time,
	}, nil
}
