package pgvalue

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
)

// Date truncates to the calendar day the DATE columns store.
func Date(value time.Time) pgtype.Date {
	return pgtype.Date{Time: mealdom.DayOf(value), Valid: true}
}
