package meal

import "time"

const dayDuration = 24 * time.Hour

// DayOf is the calendar day a timestamp belongs to. Meals, summaries and rules
// are all keyed by that day in UTC, so every package that groups by day has to
// truncate it the same way.
func DayOf(date time.Time) time.Time {
	return date.UTC().Truncate(dayDuration)
}
