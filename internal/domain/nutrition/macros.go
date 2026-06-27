// Package nutrition содержит общие типы БЖУ/нутриентов и простые операции:
// сложение макросов, проверка диапазонов, конвертация соли в натрий, расчёт ккал.
// Не зависит от инфраструктуры (HTTP, Postgres, LLM).
package nutrition

// Macros — белки/жиры/углеводы/ккал за порцию или день.
type Macros struct {
	ProteinG float64
	FatG     float64
	CarbG    float64
	Kcal     int
}

// Add складывает два набора макросов. TODO: покрыть тестами (ARCHITECTURE §16).
func (m Macros) Add(o Macros) Macros {
	return Macros{m.ProteinG + o.ProteinG, m.FatG + o.FatG, m.CarbG + o.CarbG, m.Kcal + o.Kcal}
}

// SodiumMgFromSaltG конвертирует соль (г) в натрий (мг): натрий = соль/2.54*1000 (NNR).
func SodiumMgFromSaltG(saltG float64) float64 { return saltG / 2.54 * 1000 }
