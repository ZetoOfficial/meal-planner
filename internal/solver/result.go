package solver

import planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"

// Result — результат работы солвера: план + диагностика.
type Result struct {
	Plan planningdom.Plan
}
