package quality

// Ruleset — набор quality-правил (конфиг). TODO: fiber, sodium, sat fat, sugars,
// fruit/veg, whole grain, legumes, red/processed meat, UPF (SPEC §3.2).
type Ruleset struct{}

// DefaultRuleset — стартовый набор who_nnr_dga_2025. TODO: заполнить.
func DefaultRuleset() Ruleset { return Ruleset{} }
