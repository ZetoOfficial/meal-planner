package recipe

// candidateSchema is the only shape the model may answer with: the generator
// forces structured output through this schema so Go can validate every field
// instead of parsing prose.
const candidateSchema = `{
  "type": "object",
  "properties": {
    "recipes": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "component_kind": {"type": "string", "enum": ["protein", "carb_side", "vegetable_side", "sauce", "mixed_main"]},
          "allowed_meal_types": {"type": "array", "items": {"type": "string", "enum": ["breakfast", "lunch", "dinner", "snack"]}},
          "base_servings": {"type": "integer", "minimum": 1},
          "expected_output_g": {"type": "number"},
          "cuisine": {"type": "string"},
          "cooking_method": {"type": "string"},
          "storage": {"type": "string", "enum": ["keeps_3d", "keeps_3d_reheat", "assemble_fresh", "eat_same_day"]},
          "storage_notes": {"type": "string"},
          "equipment": {"type": "array", "items": {"type": "string"}},
          "instructions": {"type": "string"},
          "slots": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "role_tag_code": {"type": "string"},
                "label": {"type": "string"},
                "qty_g": {"type": "number"},
                "prepared_qty_factor": {"type": "number"},
                "optional": {"type": "boolean"}
              },
              "required": ["role_tag_code", "label", "qty_g"]
            }
          }
        },
        "required": ["name", "component_kind", "allowed_meal_types", "base_servings", "storage", "slots"]
      }
    }
  },
  "required": ["recipes"]
}`

type candidatePayload struct {
	Recipes []candidate `json:"recipes"`
}

type candidate struct {
	Name             string          `json:"name"`
	ComponentKind    string          `json:"component_kind"`
	AllowedMealTypes []string        `json:"allowed_meal_types"`
	BaseServings     int             `json:"base_servings"`
	ExpectedOutputG  *float64        `json:"expected_output_g"`
	Cuisine          *string         `json:"cuisine"`
	CookingMethod    *string         `json:"cooking_method"`
	Storage          string          `json:"storage"`
	StorageNotes     *string         `json:"storage_notes"`
	Equipment        []string        `json:"equipment"`
	Instructions     *string         `json:"instructions"`
	Slots            []candidateSlot `json:"slots"`
}

type candidateSlot struct {
	RoleTagCode       string   `json:"role_tag_code"`
	Label             string   `json:"label"`
	QtyG              float64  `json:"qty_g"`
	PreparedQtyFactor *float64 `json:"prepared_qty_factor"`
	Optional          bool     `json:"optional"`
}
