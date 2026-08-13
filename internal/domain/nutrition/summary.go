package nutrition

type SummaryBasis string

const (
	SummaryBasisPlanned SummaryBasis = "planned"
	SummaryBasisActual  SummaryBasis = "actual"
)

type DaySummary struct {
	Basis        SummaryBasis
	Macros       Macros
	Totals       DailyTotals
	Shares       DailyShares
	Coverage     Coverage
	RuleCoverage map[string]Coverage
}

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

	GeneratedProcessedMeatG float64
	FixedProcessedMeatG     float64
}

type DailyShares struct {
	UPFKcal *float64
}

func (s DailyShares) UPFKcalShare(totalKcal int) *float64 {
	if s.UPFKcal == nil || totalKcal <= 0 {
		return nil
	}
	share := *s.UPFKcal / float64(totalKcal)
	return &share
}

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
