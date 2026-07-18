package postgres

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/mocks"
)

func TestTxManagerWithTxCommitsAfterSuccessfulCallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	beginner := mocks.NewMocktxBeginner(ctrl)
	tx := mocks.NewMockTx(ctrl)
	manager := newTxManager(beginner)
	called := 0

	gomock.InOrder(
		beginner.EXPECT().Begin(ctx).Return(tx, nil),
		tx.EXPECT().Commit(ctx).Return(nil),
	)

	err := manager.WithTx(ctx, func(context.Context) error {
		called++
		return nil
	})
	if err != nil {
		t.Fatalf("WithTx() error = %v, want nil", err)
	}
	if called != 1 {
		t.Fatalf("callback calls = %d, want 1", called)
	}
}

func TestTxManagerWithTxRollsBackAndReturnsCallbackError(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	beginner := mocks.NewMocktxBeginner(ctrl)
	tx := mocks.NewMockTx(ctrl)
	manager := newTxManager(beginner)
	want := errors.New("callback failed")
	called := 0

	gomock.InOrder(
		beginner.EXPECT().Begin(ctx).Return(tx, nil),
		tx.EXPECT().Rollback(ctx).Return(nil),
	)

	err := manager.WithTx(ctx, func(context.Context) error {
		called++
		return want
	})

	if !errors.Is(err, want) {
		t.Fatalf("WithTx() error = %v, want %v", err, want)
	}
	if called != 1 {
		t.Fatalf("callback calls = %d, want 1", called)
	}
}

func TestTxManagerWithTxRollsBackOnCallbackPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	beginner := mocks.NewMocktxBeginner(ctrl)
	tx := mocks.NewMockTx(ctrl)
	manager := newTxManager(beginner)
	panicValue := errors.New("callback panic")

	gomock.InOrder(
		beginner.EXPECT().Begin(ctx).Return(tx, nil),
		tx.EXPECT().Rollback(ctx).Return(nil),
	)

	func() {
		defer func() {
			recovered, ok := recover().(error)
			if !ok || !errors.Is(recovered, panicValue) {
				t.Fatalf("panic = %v, want %v", recovered, panicValue)
			}
		}()

		_ = manager.WithTx(ctx, func(context.Context) error {
			panic(panicValue)
		})
	}()
}

func TestTxManagerWithTxJoinsCallbackAndRollbackErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	beginner := mocks.NewMocktxBeginner(ctrl)
	tx := mocks.NewMockTx(ctrl)
	manager := newTxManager(beginner)
	callbackErr := errors.New("callback failed")
	rollbackErr := errors.New("rollback failed")

	gomock.InOrder(
		beginner.EXPECT().Begin(ctx).Return(tx, nil),
		tx.EXPECT().Rollback(ctx).Return(rollbackErr),
	)

	err := manager.WithTx(ctx, func(context.Context) error {
		return callbackErr
	})

	if !errors.Is(err, callbackErr) {
		t.Fatalf("WithTx() error = %v, want callback error", err)
	}
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("WithTx() error = %v, want rollback error", err)
	}
}

func TestTxManagerWithTxRollsBackAfterCommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	beginner := mocks.NewMocktxBeginner(ctrl)
	tx := mocks.NewMockTx(ctrl)
	manager := newTxManager(beginner)
	want := errors.New("commit failed")

	gomock.InOrder(
		beginner.EXPECT().Begin(ctx).Return(tx, nil),
		tx.EXPECT().Commit(ctx).Return(want),
		tx.EXPECT().Rollback(ctx).Return(nil),
	)

	err := manager.WithTx(ctx, func(context.Context) error { return nil })

	if !errors.Is(err, want) {
		t.Fatalf("WithTx() error = %v, want %v", err, want)
	}
}

func TestTxManagerWithTxReturnsBeginErrorWithoutInvokingCallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	beginner := mocks.NewMocktxBeginner(ctrl)
	manager := newTxManager(beginner)
	want := errors.New("begin failed")

	beginner.EXPECT().Begin(ctx).Return(nil, want)

	err := manager.WithTx(ctx, func(context.Context) error {
		t.Fatal("callback was invoked after Begin failed")
		return nil
	})

	if !errors.Is(err, want) {
		t.Fatalf("WithTx() error = %v, want %v", err, want)
	}
}

func TestStoreQueriesUseDefaultExecutor(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	db := mocks.NewMockDBTX(ctrl)
	row := mocks.NewMockRow(ctrl)
	store := NewStore(db)
	want := errors.New("row scan failed")

	db.EXPECT().QueryRow(ctx, gomock.Any(), int64(42)).Return(row)
	row.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(want)

	_, err := store.Queries(ctx).GetEater(ctx, 42)

	if !errors.Is(err, want) {
		t.Fatalf("GetEater() error = %v, want %v", err, want)
	}
}

func TestStoreQueriesUseContextBoundTransactionExecutor(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	defaultDB := mocks.NewMockDBTX(ctrl)
	tx := mocks.NewMockTx(ctrl)
	row := mocks.NewMockRow(ctrl)
	store := NewStore(defaultDB)
	want := errors.New("row scan failed")
	txCtx := withExecutor(ctx, tx)

	tx.EXPECT().QueryRow(txCtx, gomock.Any(), int64(42)).Return(row)
	row.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(want)

	_, err := store.Queries(txCtx).GetEater(txCtx, 42)

	if !errors.Is(err, want) {
		t.Fatalf("GetEater() error = %v, want %v", err, want)
	}
}

func TestTxManagerBindsTransactionExecutorForStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	defaultDB := mocks.NewMockDBTX(ctrl)
	beginner := mocks.NewMocktxBeginner(ctrl)
	tx := mocks.NewMockTx(ctrl)
	row := mocks.NewMockRow(ctrl)
	store := NewStore(defaultDB)
	manager := newTxManager(beginner)
	want := errors.New("row scan failed")

	beginner.EXPECT().Begin(ctx).Return(tx, nil)
	tx.EXPECT().QueryRow(gomock.Any(), gomock.Any(), int64(42)).Return(row)
	expectProductScan(row, want)
	tx.EXPECT().Rollback(ctx).Return(nil)

	err := manager.WithTx(ctx, func(txCtx context.Context) error {
		_, err := store.Queries(txCtx).GetProduct(txCtx, 42)
		return err
	})

	if !errors.Is(err, want) {
		t.Fatalf("WithTx() error = %v, want %v", err, want)
	}
}

func expectProductScan(row *mocks.MockRow, err error) {
	row.EXPECT().Scan(
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(err)
}
