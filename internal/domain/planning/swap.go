package planning

type SwapLevel string

const (
	SwapFill     SwapLevel = "fill"
	SwapRecipe   SwapLevel = "recipe"
	SwapRotation SwapLevel = "rotation"
)

type SwapPreview struct {
	Level   SwapLevel
	Summary string
}
