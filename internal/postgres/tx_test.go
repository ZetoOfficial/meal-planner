package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	planninguc "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
)

func TestTxManagerWithinTxCommitsOnSuccess(t *testing.T) {
	tx := &fakeTx{}
	manager := newTxManagerWithBeginner(&fakeBeginner{tx: tx})

	err := manager.WithinTx(context.Background(), func(ctx context.Context, repos planninguc.Repositories) error {
		if repos.Plans == nil || repos.Products == nil || repos.Eaters == nil || repos.Recipes == nil {
			t.Fatal("expected transaction-backed repositories")
		}

		_, _ = repos.Products.Get(ctx, 42)
		return nil
	})

	if err != nil {
		t.Fatalf("WithinTx returned error: %v", err)
	}
	if tx.commits != 1 {
		t.Fatalf("expected 1 commit, got %d", tx.commits)
	}
	if tx.rollbacks != 0 {
		t.Fatalf("expected 0 rollbacks, got %d", tx.rollbacks)
	}
	if tx.queryRows != 1 {
		t.Fatalf("expected repository query to use transaction executor, got %d calls", tx.queryRows)
	}
}

func TestTxManagerWithinTxRollsBackOnCallbackError(t *testing.T) {
	tx := &fakeTx{}
	manager := newTxManagerWithBeginner(&fakeBeginner{tx: tx})
	wantErr := errors.New("callback failed")

	err := manager.WithinTx(context.Background(), func(context.Context, planninguc.Repositories) error {
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
	if tx.commits != 0 {
		t.Fatalf("expected 0 commits, got %d", tx.commits)
	}
	if tx.rollbacks != 1 {
		t.Fatalf("expected 1 rollback, got %d", tx.rollbacks)
	}
}

type fakeBeginner struct {
	tx       *fakeTx
	beginErr error
	begins   int
}

func (b *fakeBeginner) Begin(context.Context) (pgx.Tx, error) {
	b.begins++
	if b.beginErr != nil {
		return nil, b.beginErr
	}
	return b.tx, nil
}

type fakeTx struct {
	commits   int
	rollbacks int
	queryRows int
}

func (tx *fakeTx) Begin(context.Context) (pgx.Tx, error) {
	return &fakeTx{}, nil
}

func (tx *fakeTx) Commit(context.Context) error {
	tx.commits++
	return nil
}

func (tx *fakeTx) Rollback(context.Context) error {
	tx.rollbacks++
	return nil
}

func (tx *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (tx *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *fakeTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}

func (tx *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (tx *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("query not implemented")
}

func (tx *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row {
	tx.queryRows++
	return fakeRow{}
}

func (tx *fakeTx) Conn() *pgx.Conn {
	return nil
}

type fakeRow struct{}

func (fakeRow) Scan(...any) error {
	return errors.New("scan not implemented")
}
