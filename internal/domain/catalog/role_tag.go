package catalog

type RoleKind string

const (
	RoleKindProtein RoleKind = "protein"
	RoleKindCarb    RoleKind = "carb"
	RoleKindVeg     RoleKind = "veg"
	RoleKindSauce   RoleKind = "sauce"
	RoleKindFat     RoleKind = "fat"
	RoleKindOther   RoleKind = "other"
)

type RoleTag struct {
	ID       int64
	Code     string
	Kind     RoleKind
	Label    string
	ParentID *int64
}
