package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpapi "github.com/ZetoOfficial/meal-planner/internal/api/http"
	"github.com/ZetoOfficial/meal-planner/internal/app"
	"github.com/ZetoOfficial/meal-planner/internal/clock"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	catalogrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/catalog"
	eaterrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/eater"
	planningrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/planning"
	reciperepo "github.com/ZetoOfficial/meal-planner/internal/postgres/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
	usecasecatalog "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
	usecaseeater "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
	usecaseplanning "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
	usecaserecipe "github.com/ZetoOfficial/meal-planner/internal/usecase/recipe"
)

const readHeaderTimeout = 10 * time.Second

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("fatal", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := app.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}

	store := postgres.NewStore(pool)
	tx := postgres.NewTxManager(pool)
	eaters := eaterrepo.New(store)
	products := catalogrepo.New(store)
	recipes := reciperepo.New(store)
	handler := httpapi.New(httpapi.Deps{
		APIToken: cfg.APIToken,
		Catalog:  usecasecatalog.NewService(products, tx),
		Eater:    usecaseeater.NewService(eaters, tx, clock.System{}),
		Recipe:   usecaserecipe.NewService(recipes, tx, clock.System{}),
		Planning: usecaseplanning.NewService(
			planningrepo.New(store), eaters, products, recipes, solver.New(), tx,
			usecaseplanning.Options{
				AvailableEquipment: cfg.PlannerEquipment,
				RollingWindowDays:  cfg.RollingWindowDays,
			},
		),
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	go func() {
		slog.Info("http listening", slog.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", slog.Any("error", err))
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownWait)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
