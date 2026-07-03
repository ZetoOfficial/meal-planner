package repositories

import "github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"

// LLMRepository will own LLM call logs and cache queries when that port exists.
type LLMRepository struct {
	q sqlc.Querier
}

func NewLLMRepository(q sqlc.Querier) *LLMRepository {
	return &LLMRepository{q: q}
}
