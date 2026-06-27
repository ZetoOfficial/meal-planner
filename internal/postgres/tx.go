package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TxManager реализует транзакционную границу для use case layer (ARCHITECTURE §11).
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager создаёт менеджер транзакций.
func NewTxManager(pool *pgxpool.Pool) *TxManager { return &TxManager{pool: pool} }

// WithTx выполняет fn внутри транзакции: commit при nil-ошибке, иначе rollback.
// TODO: прокидывать pgx.Tx в контекст, чтобы репозитории его подхватывали.
func (m *TxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(ctx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
