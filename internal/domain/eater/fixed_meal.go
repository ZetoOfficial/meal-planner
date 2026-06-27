package eater

// FixedMeal — фиксированный приём (напр. Сашин завтрак). Не генерируется и не
// свопается, но учитывается в БЖУ, quality-аналитике и warnings (ARCHITECTURE §5.3).
type FixedMeal struct {
	EaterID ID
	Label   string
	// TODO: макросы + hot-path нутриенты + граммаж мяса/злаков/овощей + флаги учёта.
}
