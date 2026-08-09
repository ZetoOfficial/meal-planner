package nutrition

// SummaryBasis identifies whether a summary was computed from planned or actual data.
type SummaryBasis string

const (
	SummaryBasisPlanned SummaryBasis = "planned"
	SummaryBasisActual  SummaryBasis = "actual"
)

// DaySummary aggregates macro and quality inputs for a single eater/day.
// All nutrient and food-group values are prepared-equivalent amounts attributed
// to the eater for that day. NULLable values represent a known value; absence of
// a pointer represents unknown when the value cannot be inferred from available data.
type DaySummary struct {
	Basis    SummaryBasis
	Macros   Macros
	Totals   DailyTotals
	Shares   DailyShares
	Coverage Coverage
	// RuleCoverage records per-rule known/total weights used by the evaluator.
	RuleCoverage map[string]Coverage
}

// DailyTotals collects hot-path nutrients and food-group volumes.
type DailyTotals struct {
	SaturatedFatG float64
	TransFatG     float64
	FiberG        float64
	SodiumMg      float64
	AddedSugarsG  float64
	FreeSugarsG   float64

	FruitVegG        float64
	GrainGEquiv      float64
	WholeGrainGEquiv float64
	LegumeG          float64
	RedMeatG         float64
	ProcessedMeatG   float64

	// GeneratedProcessedMeatG and FixedProcessedMeatG split ProcessedMeatG by source.
	// The weekly processed-meat rule treats generated contribution as a soft_fail
	// and fixed-meal contribution as a warn.
	GeneratedProcessedMeatG float64
	FixedProcessedMeatG     float64
}

// DailyShares collects fractional quality metrics.
type DailyShares struct {
	// UPFKcal is the absolute kilocalories from ultra-processed foods. The share
	// is computed by the evaluator against the day's total kcal. A nil pointer
	// means the share cannot be reliably computed.
	UPFKcal *float64
}

// UPFKcalShare returns the UPF kcal share when both numerator and denominator
// are known and the denominator is positive. Otherwise it returns nil.
func (s DailyShares) UPFKcalShare(totalKcal int) *float64 {
	if s.UPFKcal == nil || totalKcal <= 0 {
		return nil
	}
	share := *s.UPFKcal / float64(totalKcal)
	return &share
}

// RollingSummary aggregates day summaries over a window for weekly rules.
type RollingSummary struct {
	Basis                   SummaryBasis
	WindowDays              int
	RedMeatG                float64
	ProcessedMeatG          float64
	GeneratedProcessedMeatG float64
	FixedProcessedMeatG     float64
	LegumeServings          float64
	FiberGAvg               float64
	SodiumMgAvg             float64
	SaturatedFatGAvg        float64
	AddedSugarsGAvg         float64
	FruitVegGAvg            float64
	Coverage                Coverage
	RuleCoverage            map[string]Coverage
}
