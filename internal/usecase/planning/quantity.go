package planning

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	"github.com/ZetoOfficial/meal-planner/internal/swap"
)

// QuantityCommand locks one slot fill to a measured quantity before cooking.
type QuantityCommand struct {
	SlotFillID       planningdom.SlotFillID
	PlannedInputQtyG float64
}

// PreviewQuantityAdjustment shows what the locked quantity would do, without
// touching the plan.
func (s *Service) PreviewQuantityAdjustment(
	ctx context.Context,
	planID planningdom.PlanID,
	command QuantityCommand,
) (swap.QuantityPreview, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return swap.QuantityPreview{}, err
	}
	return s.quantityPreview(ctx, plan, command)
}

// ApplyQuantityAdjustment writes the accepted preview atomically: the fill
// quantity, the rebalanced allocations, the quality snapshot and the shopping
// list state move together or not at all.
func (s *Service) ApplyQuantityAdjustment(
	ctx context.Context,
	planID planningdom.PlanID,
	command QuantityCommand,
	confirmed bool,
) (PlanResult, error) {
	var result PlanResult
	err := s.transactions.WithTx(ctx, func(ctx context.Context) error {
		plan, err := s.plans.Get(ctx, planID)
		if err != nil {
			return err
		}
		preview, err := s.quantityPreview(ctx, plan, command)
		if err != nil {
			return err
		}
		if !confirmed {
			return &UnconfirmedQuantityError{Preview: preview}
		}

		snapshots := quantitySnapshots(preview)
		// The stale flag is part of the plan ReplacePlanAllocations reads back,
		// so it has to be set before the plan is re-read for the response.
		if err := s.plans.MarkShoppingListStale(ctx, planID, true); err != nil {
			return err
		}
		saved, err := s.plans.ReplacePlanAllocations(ctx, planID, preview.Plan)
		if err != nil {
			return err
		}
		if err := s.plans.SaveQualitySnapshots(ctx, planID, snapshots); err != nil {
			return err
		}
		result = PlanResult{Plan: saved, Snapshots: snapshots}
		return nil
	})
	if err != nil {
		return PlanResult{}, err
	}
	return result, nil
}

// UnconfirmedQuantityError carries the preview the owner still has to accept.
type UnconfirmedQuantityError struct {
	Preview swap.QuantityPreview
}

func (e *UnconfirmedQuantityError) Error() string {
	return "quantity adjustment needs explicit confirmation"
}

func (s *Service) quantityPreview(
	ctx context.Context,
	plan planningdom.Plan,
	command QuantityCommand,
) (swap.QuantityPreview, error) {
	input, err := s.swapInput(ctx, plan)
	if err != nil {
		return swap.QuantityPreview{}, err
	}
	goals, err := s.planGoals(ctx, plan)
	if err != nil {
		return swap.QuantityPreview{}, err
	}

	preview, err := swap.ComputeQuantity(input, goals, swap.QuantityRequest{
		SlotFillID:       command.SlotFillID,
		PlannedInputQtyG: command.PlannedInputQtyG,
	})
	if err != nil {
		return swap.QuantityPreview{}, err
	}
	return preview, nil
}

// planGoals reads the exact goal version each eater-day was planned against, so
// a later profile edit cannot change what an old plan is measured by.
func (s *Service) planGoals(ctx context.Context, plan planningdom.Plan) ([]swap.GoalTarget, error) {
	byEater := make(map[eaterdom.ID][]eaterdom.MacroGoal)
	goals := make([]swap.GoalTarget, 0, len(plan.EaterDays))
	for i := range plan.EaterDays {
		day := &plan.EaterDays[i]
		history, loaded := byEater[day.EaterID]
		if !loaded {
			var err error
			history, err = s.eaters.ListGoals(ctx, day.EaterID)
			if err != nil {
				return nil, err
			}
			byEater[day.EaterID] = history
		}
		goal := goalByID(history, day.MacroGoalID)
		if goal == nil {
			return nil, apperr.Conflict("the macro goal a plan day was built on no longer exists")
		}
		goals = append(goals, swap.GoalTarget{EaterID: day.EaterID, Date: day.Date, Range: goal.Range})
	}
	return goals, nil
}

func goalByID(goals []eaterdom.MacroGoal, id eaterdom.MacroGoalID) *eaterdom.MacroGoal {
	for i := range goals {
		if goals[i].ID == id {
			return &goals[i]
		}
	}
	return nil
}

func quantitySnapshots(preview swap.QuantityPreview) []planningdom.QualitySnapshot {
	snapshots := make([]planningdom.QualitySnapshot, 0, len(preview.EaterDays))
	for i := range preview.EaterDays {
		day := &preview.EaterDays[i]
		snapshots = append(snapshots, planningdom.QualitySnapshot{
			EaterID:      day.EaterID,
			Date:         day.Date,
			Basis:        day.Basis,
			Day:          day.Day,
			Rolling:      day.Rolling,
			DayRules:     day.DayRules,
			RollingRules: day.RollingRules,
		})
	}
	return snapshots
}
