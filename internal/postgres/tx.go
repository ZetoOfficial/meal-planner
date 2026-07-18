package postgres

//go:generate go tool mockgen -source=$GOFILE -imports=pgx=github.com/jackc/pgx/v5 -destination=mocks/transaction.go -package=mocks

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

type TxManager struct {
	beginner txBeginner
}

func NewTxManager(pool *pgxpool.Pool) *TxManager { return newTxManager(pool) }

func newTxManager(beginner txBeginner) *TxManager {
	return &TxManager{beginner: beginner}
}

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
	if callbackErr := fn(withExecutor(ctx, tx)); callbackErr != nil {
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
