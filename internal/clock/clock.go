// Package clock абстрагирует время для тестируемости (ARCHITECTURE §4).
package clock

import "time"

// Clock отдаёт текущее время.
type Clock interface{ Now() time.Time }

// System — реальные часы.
type System struct{}

// Now возвращает time.Now().
func (System) Now() time.Time { return time.Now() }
