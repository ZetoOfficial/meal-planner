package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/repositories"
)

func TestTxManagerWithTxCommitsAfterSuccessfulCallback(t *testing.T) {
	tx := &fakeTx{}
	beginner := &fakeBeginner{tx: tx}
	manager := newTxManager(beginner)
	called := 0

	err := manager.WithTx(context.Background(), func(context.Context) error {
		called++
		return nil
	})

	if err != nil {
		t.Fatalf("WithTx() error = %v, want nil", err)
	}
	if called != 1 {
		t.Fatalf("callback calls = %d, want 1", called)
	}
	if beginner.beginCalls != 1 {
		t.Fatalf("begin calls = %d, want 1", beginner.beginCalls)
	}
	if tx.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", tx.commitCalls)
	}
	if tx.rollbackCalls != 0 {
		t.Fatalf("rollback calls = %d, want 0", tx.rollbackCalls)
	}
}

func TestTxManagerWithTxRollsBackAndReturnsCallbackError(t *testing.T) {
	tx := &fakeTx{}
	manager := newTxManager(&fakeBeginner{tx: tx})
	want := errors.New("callback failed")
	called := 0

	err := manager.WithTx(context.Background(), func(context.Context) error {
		called++
		return want
	})

	if !errors.Is(err, want) {
		t.Fatalf("WithTx() error = %v, want %v", err, want)
	}
	if called != 1 {
		t.Fatalf("callback calls = %d, want 1", called)
	}
	if tx.commitCalls != 0 {
		t.Fatalf("commit calls = %d, want 0", tx.commitCalls)
	}
	if tx.rollbackCalls != 1 {
		t.Fatalf("rollback calls = %d, want 1", tx.rollbackCalls)
	}
}

func TestTxManagerWithTxRollsBackOnCallbackPanic(t *testing.T) {
	tx := &fakeTx{}
	manager := newTxManager(&fakeBeginner{tx: tx})
	panicValue := errors.New("callback panic")

	func() {
		defer func() {
			if recovered := recover(); recovered != panicValue {
				t.Fatalf("panic = %v, want %v", recovered, panicValue)
			}
		}()

		_ = manager.WithTx(context.Background(), func(context.Context) error {
			panic(panicValue)
		})
	}()

	if tx.rollbackCalls != 1 {
		t.Fatalf("rollback calls = %d, want 1", tx.rollbackCalls)
	}
}

func TestTxManagerWithTxJoinsCallbackAndRollbackErrors(t *testing.T) {
	callbackErr := errors.New("callback failed")
	rollbackErr := errors.New("rollback failed")
	tx := &fakeTx{rollbackErr: rollbackErr}
	manager := newTxManager(&fakeBeginner{tx: tx})

	err := manager.WithTx(context.Background(), func(context.Context) error {
		return callbackErr
	})

	if !errors.Is(err, callbackErr) {
		t.Fatalf("WithTx() error = %v, want callback error", err)
	}
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("WithTx() error = %v, want rollback error", err)
	}
}

func TestTxManagerWithTxReturnsCommitError(t *testing.T) {
	want := errors.New("commit failed")
	tx := &fakeTx{commitErr: want}
	manager := newTxManager(&fakeBeginner{tx: tx})

	err := manager.WithTx(context.Background(), func(context.Context) error { return nil })

	if !errors.Is(err, want) {
		t.Fatalf("WithTx() error = %v, want %v", err, want)
	}
	if tx.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", tx.commitCalls)
	}
	if tx.rollbackCalls != 1 {
		t.Fatalf("rollback calls = %d, want 1", tx.rollbackCalls)
	}
}

func TestTxManagerWithTxReturnsBeginErrorWithoutInvokingCallback(t *testing.T) {
	want := errors.New("begin failed")
	beginner := &fakeBeginner{beginErr: want}
	manager := newTxManager(beginner)
	called := false

	err := manager.WithTx(context.Background(), func(context.Context) error {
		called = true
		return nil
	})

	if !errors.Is(err, want) {
		t.Fatalf("WithTx() error = %v, want %v", err, want)
	}
	if called {
		t.Fatal("callback was invoked after Begin failed")
	}
	if beginner.beginCalls != 1 {
		t.Fatalf("begin calls = %d, want 1", beginner.beginCalls)
	}
}

func TestRepositoriesQueriesUseProvidedExecutor(t *testing.T) {
	db := &fakeDBTX{}
	repos := repositories.New(db)

	_, err := repos.Queries(context.Background()).GetEater(context.Background(), 42)

	if !errors.Is(err, errFakeRowScan) {
		t.Fatalf("GetEater() error = %v, want %v", err, errFakeRowScan)
	}
	if db.queryRowCalls != 1 {
		t.Fatalf("QueryRow calls = %d, want 1", db.queryRowCalls)
	}
	if !strings.Contains(db.lastQuery, "FROM eater") {
		t.Fatalf("query = %q, want generated eater query", db.lastQuery)
	}
	if len(db.lastArgs) != 1 || db.lastArgs[0] != int64(42) {
		t.Fatalf("query args = %#v, want [42]", db.lastArgs)
	}
}

func TestRepositoriesQueriesUseContextBoundTransactionExecutor(t *testing.T) {
	defaultDB := &fakeDBTX{}
	tx := &fakeTx{}
	repos := repositories.New(defaultDB)
	ctx := repositories.WithExecutor(context.Background(), tx)

	_, err := repos.Queries(ctx).GetEater(ctx, 42)

	if !errors.Is(err, errFakeRowScan) {
		t.Fatalf("GetEater() error = %v, want %v", err, errFakeRowScan)
	}
	if tx.queryRowCalls != 1 {
		t.Fatalf("transaction QueryRow calls = %d, want 1", tx.queryRowCalls)
	}
	if defaultDB.queryRowCalls != 0 {
		t.Fatalf("default QueryRow calls = %d, want 0", defaultDB.queryRowCalls)
	}
}

func TestTxManagerBindsTransactionExecutorForRepositories(t *testing.T) {
	defaultDB := &fakeDBTX{}
	tx := &fakeTx{}
	repos := repositories.New(defaultDB)
	manager := newTxManager(&fakeBeginner{tx: tx})

	err := manager.WithTx(context.Background(), func(ctx context.Context) error {
		_, err := repos.Queries(ctx).GetProduct(ctx, 42)
		return err
	})

	if !errors.Is(err, errFakeRowScan) {
		t.Fatalf("WithTx() error = %v, want %v", err, errFakeRowScan)
	}
	if tx.queryRowCalls != 1 {
		t.Fatalf("transaction QueryRow calls = %d, want 1", tx.queryRowCalls)
	}
	if defaultDB.queryRowCalls != 0 {
		t.Fatalf("default QueryRow calls = %d, want 0", defaultDB.queryRowCalls)
	}
}

type fakeBeginner struct {
	tx         pgx.Tx
	beginErr   error
	beginCalls int
}

func (f *fakeBeginner) Begin(context.Context) (pgx.Tx, error) {
	f.beginCalls++
	return f.tx, f.beginErr
}

type fakeTx struct {
	pgx.Tx
	commitCalls   int
	rollbackCalls int
	commitErr     error
	rollbackErr   error
	queryRowCalls int
}

func (f *fakeTx) Commit(context.Context) error {
	f.commitCalls++
	return f.commitErr
}

func (f *fakeTx) Rollback(context.Context) error {
	f.rollbackCalls++
	return f.rollbackErr
}

func (f *fakeTx) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	f.queryRowCalls++
	return fakeRow{}
}

var errFakeRowScan = errors.New("fake row scan")

type fakeDBTX struct {
	queryRowCalls int
	lastQuery     string
	lastArgs      []interface{}
}

func (*fakeDBTX) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (*fakeDBTX) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (f *fakeDBTX) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	f.queryRowCalls++
	f.lastQuery = query
	f.lastArgs = args
	return fakeRow{}
}

type fakeRow struct{}

func (fakeRow) Scan(...any) error { return errFakeRowScan }
