package planning

import (
	"context"
	"errors"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
	"github.com/ZetoOfficial/meal-planner/internal/swap"
	"github.com/ZetoOfficial/meal-planner/internal/usecase/reference"
)

// SwapCommand asks to replace the product of one stored slot fill. Recipe and
// rotation level swaps need a full re-solve and are not part of v1.
type SwapCommand struct {
	SlotFillID   planningdom.SlotFillID
	NewProductID catalogdom.ProductID
}

// PreviewSwap computes the blast radius without changing anything.
func (s *Service) PreviewSwap(
	ctx context.Context,
	planID planningdom.PlanID,
	command SwapCommand,
) (swap.Preview, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return swap.Preview{}, err
	}
	input, err := s.swapInput(ctx, plan)
	if err != nil {
		return swap.Preview{}, err
	}

	preview, err := swap.Compute(input, swap.Request{
		SlotFillID:   command.SlotFillID,
		NewProductID: command.NewProductID,
	})
	if err != nil {
		return swap.Preview{}, swapRequestError("swap cannot be previewed", err)
	}
	return preview, nil
}

// ApplySwap writes a confirmed swap and re-snapshots the plan's quality, so the
// stored explanation keeps matching the stored plan. Breaking the prep graph
// needs an explicit confirmation because dependent dishes stop being valid.
func (s *Service) ApplySwap(
	ctx context.Context,
	planID planningdom.PlanID,
	command SwapCommand,
	confirmed bool,
) (PlanResult, error) {
	var result PlanResult
	err := s.transactions.WithTx(ctx, func(ctx context.Context) error {
		plan, err := s.plans.Get(ctx, planID)
		if err != nil {
			return err
		}
		input, err := s.swapInput(ctx, plan)
		if err != nil {
			return err
		}
		preview, err := swap.Compute(input, swap.Request{
			SlotFillID:   command.SlotFillID,
			NewProductID: command.NewProductID,
		})
		if err != nil {
			return swapRequestError("swap cannot be applied", err)
		}
		if !confirmed {
			return &UnconfirmedSwapError{Preview: preview}
		}

		if err := s.plans.SwapSlotFill(ctx, planID, command.SlotFillID, command.NewProductID); err != nil {
			return err
		}
		snapshots := swapSnapshots(preview)
		if err := s.plans.SaveQualitySnapshots(ctx, planID, snapshots); err != nil {
			return err
		}
		stored, err := s.plans.Get(ctx, planID)
		if err != nil {
			return err
		}
		result = PlanResult{Plan: stored, Snapshots: snapshots}
		return nil
	})
	if err != nil {
		return PlanResult{}, err
	}
	return result, nil
}

// swapRequestError reports only the rejections that describe a bad request as
// validation failures. A plan that no longer matches the catalogue is corrupt
// data: turning it into 422 would blame the owner for a storage problem.
func swapRequestError(message string, err error) error {
	switch {
	case errors.Is(err, swap.ErrUnknownFill),
		errors.Is(err, swap.ErrUnknownProduct),
		errors.Is(err, swap.ErrNonPositiveQuantity):
		return apperr.Validation(message, err.Error())
	default:
		return err
	}
}

// UnconfirmedSwapError carries the preview the owner still has to accept.
type UnconfirmedSwapError struct {
	Preview swap.Preview
}

func (e *UnconfirmedSwapError) Error() string {
	return "swap needs explicit confirmation"
}

func swapSnapshots(preview swap.Preview) []planningdom.QualitySnapshot {
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

// swapInput assembles the catalogue and the rolling-window history a stored
// plan needs to be re-evaluated the same way it was built.
func (s *Service) swapInput(ctx context.Context, plan planningdom.Plan) (swap.Input, error) {
	catalogue, err := reference.LoadCatalogue(ctx, reference.Readers{
		Products: s.products,
		Recipes:  s.recipes,
		Eaters:   s.eaters,
	}, []planningdom.Plan{plan})
	if err != nil {
		return swap.Input{}, err
	}
	history := make(map[eaterdom.ID][]nutrition.DaySummary, len(plan.EaterDays))
	for i := range plan.EaterDays {
		eaterID := plan.EaterDays[i].EaterID
		if _, loaded := history[eaterID]; loaded {
			continue
		}
		days, err := s.eaterHistory(ctx, eaterID, plan.StartDate)
		if err != nil {
			return swap.Input{}, err
		}
		history[eaterID] = days
	}
	return swap.Input{
		Plan:              plan,
		Catalogue:         catalogue,
		Ruleset:           quality.DefaultRuleset(),
		RollingWindowDays: s.options.RollingWindowDays,
		History:           history,
	}, nil
}
