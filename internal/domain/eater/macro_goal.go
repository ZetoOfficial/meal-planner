package eater

import "time"

type MacroRange struct {
	ProteinMinG, ProteinMaxG float64
	FatMinG, FatMaxG         float64
	CarbMinG, CarbMaxG       float64
	KcalMin, KcalMax         int
}

type MacroGoal struct {
	EaterID       ID
	EffectiveFrom time.Time
	Range         MacroRange
}
