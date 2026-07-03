package repositories

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

type FeedbackRepository struct {
	q sqlc.Querier
}

func NewFeedbackRepository(q sqlc.Querier) *FeedbackRepository {
	return &FeedbackRepository{q: q}
}

func (r *FeedbackRepository) Save(context.Context, any) error {
	return ErrNotImplemented
}
