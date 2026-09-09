package planning

//go:generate go tool mockgen -source=$GOFILE -destination=mocks/ports.go -package=mocks

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
)

type TxRunner interface {
	WithTx(ctx context.Context, fn func(context.Context) error) error
}

// PlanRepository persists the plan aggregate. Save expects to run inside the
// caller's transaction so a failed step leaves no partial plan behind.
type PlanRepository interface {
	Save(ctx context.Context, plan planningdom.Plan) (planningdom.Plan, error)
	Get(ctx context.Context, id planningdom.PlanID) (planningdom.Plan, error)

	SaveQualitySnapshots(ctx context.Context, planID planningdom.PlanID, snapshots []planningdom.QualitySnapshot) error
	ListQualitySnapshots(ctx context.Context, planID planningdom.PlanID, basis nutrition.SummaryBasis) ([]planningdom.QualitySnapshot, error)
}
