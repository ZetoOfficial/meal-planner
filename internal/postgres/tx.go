package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/repositories"
)

type txBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// TxManager реализует транзакционную границу для use case layer (ARCHITECTURE §11).
type TxManager struct {
	beginner txBeginner
}

// NewTxManager создаёт менеджер транзакций.
func NewTxManager(pool *pgxpool.Pool) *TxManager { return newTxManager(pool) }

func newTxManager(beginner txBeginner) *TxManager {
	return &TxManager{beginner: beginner}
}

// WithTx выполняет fn внутри транзакции: commit при nil-ошибке, иначе rollback.
func (m *TxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.beginner.Begin(ctx)
	if err != nil {
		return err
	}
	finalized := false
	defer func() {
		if !finalized {
			_ = tx.Rollback(ctx)
		}
	}()
	if callbackErr := fn(repositories.WithExecutor(ctx, tx)); callbackErr != nil {
		rollbackErr := tx.Rollback(ctx)
		finalized = true
		if rollbackErr != nil {
			return errors.Join(callbackErr, rollbackErr)
		}
		return callbackErr
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	finalized = true
	return nil
}
