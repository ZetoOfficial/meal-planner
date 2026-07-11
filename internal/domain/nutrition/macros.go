package nutrition

type Macros struct {
	ProteinG float64
	FatG     float64
	CarbG    float64
	Kcal     int
}

func (m Macros) Add(o Macros) Macros {
	return Macros{m.ProteinG + o.ProteinG, m.FatG + o.FatG, m.CarbG + o.CarbG, m.Kcal + o.Kcal}
}

const mgPerG = 1000

func SodiumMgFromSaltG(saltG float64) float64 { return saltG / 2.54 * mgPerG }
