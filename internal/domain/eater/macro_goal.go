package eater

import "time"

// MacroRange — цель по БЖУ диапазонами (min–max) для Б, Ж, У и ккал.
type MacroRange struct {
	ProteinMinG, ProteinMaxG float64
	FatMinG, FatMaxG         float64
	CarbMinG, CarbMaxG       float64
	KcalMin, KcalMax         int
}

// MacroGoal — версионированная цель (текущая = макс. EffectiveFrom <= сегодня).
type MacroGoal struct {
	EaterID       ID
	EffectiveFrom time.Time
	Range         MacroRange
}
