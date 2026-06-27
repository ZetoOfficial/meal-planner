package planning

// SwapLevel — уровень свопа (влияет на blast radius).
type SwapLevel string

const (
	SwapFill     SwapLevel = "fill"
	SwapRecipe   SwapLevel = "recipe"
	SwapRotation SwapLevel = "rotation"
)

// SwapPreview — blast radius до подтверждения: diff БЖУ/качества, затронутые приёмы,
// невалидные блюда по prep-графу. TODO: поля (см. api/openapi.yaml SwapPreview).
type SwapPreview struct {
	Level   SwapLevel
	Summary string
}
