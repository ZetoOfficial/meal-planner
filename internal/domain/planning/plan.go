// Package planning — центральный доменный модуль: генерация плана, раскладка по
// датам/приёмам/едокам, planned vs actual, свопы, blast radius, prep-граф (ARCHITECTURE §5.4).
package planning

// PlanID — идентификатор плана.
type PlanID int64

// Plan — партия готовки на N дней. TODO: компоненты, meals, prep-граф, quality summary.
type Plan struct {
	ID   PlanID
	Days int
}

// AttachQuality навешивает результат quality-оценки на план (см. ARCHITECTURE §11).
func (p *Plan) AttachQuality(_ any) {}
