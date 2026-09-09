package solver

import (
	"fmt"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

const dateLayout = time.DateOnly

// DiagnosticCode names the likely reason a plan could not be built. Codes are
// stable so the API can map them without re-deriving intent from prose.
type DiagnosticCode string

const (
	DiagnosticMissingRole        DiagnosticCode = "missing_role"
	DiagnosticAllergenConflict   DiagnosticCode = "allergen_conflict"
	DiagnosticExclusionConflict  DiagnosticCode = "exclusion_conflict"
	DiagnosticStorageConflict    DiagnosticCode = "storage_conflict"
	DiagnosticEquipmentConflict  DiagnosticCode = "equipment_conflict"
	DiagnosticHardQualityFailure DiagnosticCode = "hard_quality_violation"
	DiagnosticMacroInfeasible    DiagnosticCode = "macro_infeasible"
	DiagnosticSearchLimit        DiagnosticCode = "search_limit_reached"
)

type Diagnostic struct {
	Code      DiagnosticCode
	Message   string
	EaterID   eaterdom.ID
	ProductID catalogdom.ProductID
	RecipeID  recipedom.ID
	Detail    string
}

// Diagnostics explains an impossible plan: first why the search itself ended,
// then which candidates were filtered out before it started. Order is stable
// and duplicates from many equally rejected candidates collapse into one entry.
func (f Failure) Diagnostics() []Diagnostic {
	result := []Diagnostic{f.searchDiagnostic()}
	seen := map[Diagnostic]struct{}{}
	for i := range f.Rejections {
		diagnostic := rejectionDiagnostic(f.Rejections[i])
		key := Diagnostic{Code: diagnostic.Code, EaterID: diagnostic.EaterID, Detail: diagnostic.Detail}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, diagnostic)
	}
	return result
}

func (f Failure) searchDiagnostic() Diagnostic {
	if f.Code == FailureSearchLimitReached {
		return Diagnostic{
			Code:    DiagnosticSearchLimit,
			EaterID: f.EaterID,
			Message: fmt.Sprintf(
				"search budget ran out for eater %d on %s before a macro-feasible plan was found",
				f.EaterID, f.Date.Format(dateLayout),
			),
		}
	}
	return Diagnostic{
		Code:    DiagnosticMacroInfeasible,
		EaterID: f.EaterID,
		Message: fmt.Sprintf(
			"no combination of the remaining candidates reaches the macro targets for eater %d on %s",
			f.EaterID, f.Date.Format(dateLayout),
		),
	}
}

func rejectionDiagnostic(rejection HardRejection) Diagnostic {
	diagnostic := Diagnostic{
		Code:      diagnosticCode(rejection.Code),
		EaterID:   rejection.EaterID,
		ProductID: rejection.ProductID,
		RecipeID:  rejection.RecipeID,
		Detail:    rejection.Detail,
	}
	diagnostic.Message = diagnosticMessage(diagnostic)
	return diagnostic
}

func diagnosticCode(code RejectionCode) DiagnosticCode {
	switch code {
	case RejectionAllergen:
		return DiagnosticAllergenConflict
	case RejectionHardExclusion:
		return DiagnosticExclusionConflict
	case RejectionIndustrialTransFat:
		return DiagnosticHardQualityFailure
	case RejectionStorage:
		return DiagnosticStorageConflict
	case RejectionEquipment:
		return DiagnosticEquipmentConflict
	case RejectionRoleCompatibility:
		return DiagnosticMissingRole
	default:
		return DiagnosticMacroInfeasible
	}
}

func diagnosticMessage(diagnostic Diagnostic) string {
	switch diagnostic.Code {
	case DiagnosticAllergenConflict:
		return fmt.Sprintf(
			"product %d was dropped because eater %d is allergic to %s",
			diagnostic.ProductID, diagnostic.EaterID, diagnostic.Detail,
		)
	case DiagnosticExclusionConflict:
		return fmt.Sprintf(
			"product %d was dropped by a hard exclusion of eater %d on %s",
			diagnostic.ProductID, diagnostic.EaterID, diagnostic.Detail,
		)
	case DiagnosticHardQualityFailure:
		return fmt.Sprintf(
			"product %d was dropped because of industrial trans fat (%s)",
			diagnostic.ProductID, diagnostic.Detail,
		)
	case DiagnosticStorageConflict:
		return fmt.Sprintf(
			"recipe %d cannot be cooked ahead for a multi-day plan (storage %s)",
			diagnostic.RecipeID, diagnostic.Detail,
		)
	case DiagnosticEquipmentConflict:
		return fmt.Sprintf(
			"recipe %d needs unavailable equipment %s",
			diagnostic.RecipeID, diagnostic.Detail,
		)
	case DiagnosticMissingRole:
		return fmt.Sprintf(
			"recipe %d has no allowed product for required role %s",
			diagnostic.RecipeID, diagnostic.Detail,
		)
	case DiagnosticMacroInfeasible, DiagnosticSearchLimit:
		return fmt.Sprintf("recipe %d was dropped: %s", diagnostic.RecipeID, diagnostic.Detail)
	default:
		return fmt.Sprintf("recipe %d was dropped: %s", diagnostic.RecipeID, diagnostic.Detail)
	}
}
