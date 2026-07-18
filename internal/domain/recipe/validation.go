package recipe

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
)

const (
	maxRecipeWeight = 99.999
	maxSlotQuantity = 999_999.9
)

func Validate(template Template, roles map[string]catalog.RoleTag) error {
	if err := validateTemplateFields(template); err != nil {
		return err
	}
	if err := validateEquipment(template.Equipment); err != nil {
		return err
	}
	return validateSlots(template.Slots, roles)
}

func validateTemplateFields(template Template) error {
	if strings.TrimSpace(template.Name) == "" {
		return errors.New("recipe name must not be blank")
	}
	if !validStatus(template.Status) {
		return fmt.Errorf("recipe status %q is invalid", template.Status)
	}
	if !validSource(template.Source) {
		return fmt.Errorf("recipe source %q is invalid", template.Source)
	}
	if !template.Storage.Valid() {
		return fmt.Errorf("recipe storage profile %q is invalid", template.Storage)
	}
	if math.IsNaN(template.Weight) || math.IsInf(template.Weight, 0) || template.Weight < 0 || template.Weight > maxRecipeWeight {
		return fmt.Errorf("recipe weight must be between 0 and %g", maxRecipeWeight)
	}
	if template.CookedCount < 0 || int64(template.CookedCount) > math.MaxInt32 {
		return errors.New("recipe cooked count must fit a nonnegative int32")
	}
	return nil
}

func validateEquipment(values []string) error {
	for _, equipment := range values {
		if strings.TrimSpace(equipment) == "" {
			return errors.New("recipe equipment must not be blank")
		}
	}
	return nil
}

func validateSlots(slots []Slot, roles map[string]catalog.RoleTag) error {
	if len(slots) == 0 {
		return errors.New("recipe must have at least one slot")
	}
	positions := make(map[int]struct{}, len(slots))
	for _, slot := range slots {
		if err := validateSlot(slot, roles); err != nil {
			return err
		}
		if _, exists := positions[slot.Position]; exists {
			return fmt.Errorf("recipe slot position %d is duplicated", slot.Position)
		}
		positions[slot.Position] = struct{}{}
	}
	return nil
}

func validateSlot(slot Slot, roles map[string]catalog.RoleTag) error {
	code := strings.TrimSpace(slot.RoleTagCode)
	if code == "" {
		return errors.New("recipe slot role must not be blank")
	}
	if strings.TrimSpace(slot.Label) == "" {
		return errors.New("recipe slot label must not be blank")
	}
	if math.IsNaN(slot.Qty) || math.IsInf(slot.Qty, 0) || slot.Qty <= 0 || slot.Qty > maxSlotQuantity {
		return fmt.Errorf("recipe slot quantity must be between 0 and %g", maxSlotQuantity)
	}
	if strings.TrimSpace(slot.Unit) == "" {
		return errors.New("recipe slot unit must not be blank")
	}
	if slot.Position < 0 || int64(slot.Position) > math.MaxInt32 {
		return errors.New("recipe slot position must fit a nonnegative int32")
	}
	role, ok := roles[code]
	if !ok {
		return fmt.Errorf("recipe slot role %q does not exist", code)
	}
	if !role.MatchesKind() {
		return fmt.Errorf("recipe slot role %q is incompatible with role kind %q", role.Code, role.Kind)
	}
	return nil
}

func validStatus(status Status) bool {
	switch status {
	case StatusDraft, StatusPermanent, StatusRejected:
		return true
	default:
		return false
	}
}

func validSource(source Source) bool {
	switch source {
	case SourceLLM, SourceManual:
		return true
	default:
		return false
	}
}
