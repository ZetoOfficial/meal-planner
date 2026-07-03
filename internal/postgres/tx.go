package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/repositories"
	planninguc "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
)

type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// TxManager implements the usecase transaction boundary.
type TxManager struct {
	db txBeginner
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return newTxManagerWithBeginner(pool)
}

func newTxManagerWithBeginner(db txBeginner) *TxManager {
	return &TxManager{db: db}
}

func (m *TxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.WithinTx(ctx, func(ctx context.Context, _ planninguc.Repositories) error {
		return fn(ctx)
	})
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context, repos planninguc.Repositories) error) error {
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return err
	}

	if err := fn(ctx, repositories.New(tx).Planning()); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}
