package catalog

// RoleKind — верхний уровень роли продукта в слоте.
type RoleKind string

// RoleTag — ролевой тег с подклассами; code = "kind:value[:subclass]".
type RoleTag struct {
	Code string
	Kind RoleKind
}
