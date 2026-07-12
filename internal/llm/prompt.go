package llm

import _ "embed"

//go:embed prompts/recipe_generation.md
var recipeGenerationPrompt string

func RecipeGenerationPrompt() string { return recipeGenerationPrompt }
