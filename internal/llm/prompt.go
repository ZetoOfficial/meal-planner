package llm

import _ "embed"

// recipeGenerationPrompt — системный промпт генерации рецептов.
// Промпт = код: версионируется в репе, тестируется на фиксированных примерах (SPEC §11).
//
//go:embed prompts/recipe_generation.md
var recipeGenerationPrompt string

// RecipeGenerationPrompt возвращает текст системного промпта.
func RecipeGenerationPrompt() string { return recipeGenerationPrompt }
