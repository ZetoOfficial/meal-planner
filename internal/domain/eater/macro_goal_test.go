package eater

import (
	"testing"
	"time"
)

func TestCurrentGoalAtIgnoresFutureGoals(t *testing.T) {
	today := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	goals := []MacroGoal{
		{ID: 1, EffectiveFrom: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
		{ID: 2, EffectiveFrom: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)},
		{ID: 3, EffectiveFrom: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)},
	}

	current := CurrentGoalAt(goals, today)
	if current == nil || current.ID != 2 {
		t.Fatalf("CurrentGoalAt() = %#v, want goal 2", current)
	}
}
