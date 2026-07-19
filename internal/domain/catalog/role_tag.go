package catalog

import "strings"

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

func (r RoleTag) MatchesKind() bool {
	prefix, _, _ := strings.Cut(r.Code, ":")
	return prefix == string(r.Kind)
}
