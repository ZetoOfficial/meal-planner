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
	"github.com/ZetoOfficial/meal-planner/internal/llm"
	"github.com/ZetoOfficial/meal-planner/internal/llm/anthropic"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	analyticsrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/analytics"
	catalogrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/catalog"
	eaterrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/eater"
	feedbackrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/feedback"
	llmrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/llm"
	planningrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/planning"
	reciperepo "github.com/ZetoOfficial/meal-planner/internal/postgres/recipe"
	shoppingrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/shopping"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
	usecasecatalog "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
	usecaseeater "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
	usecaseplanning "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
	usecasereality "github.com/ZetoOfficial/meal-planner/internal/usecase/reality"
	usecaserecipe "github.com/ZetoOfficial/meal-planner/internal/usecase/recipe"
	usecaseshopping "github.com/ZetoOfficial/meal-planner/internal/usecase/shopping"
)

const readHeaderTimeout = 10 * time.Second

// llmProvider resolves the configured provider name. The composition root owns
// this switch: the core declares the Provider interface, so it cannot import
// the implementations without a cycle.
func llmProvider(name string) (llm.Provider, error) {
	switch name {
	case anthropic.Name:
		return anthropic.New(), nil
	case llm.OpenAIName:
		return llm.NewOpenAIProvider(), nil
	default:
		return nil, fmt.Errorf("unknown llm provider %q, want %q or %q",
			name, anthropic.Name, llm.OpenAIName)
	}
}

// recipeGenerator stays nil without an API key: the planner then works
// reuse-only instead of failing at start-up.
func recipeGenerator(cfg app.Config, store *postgres.Store) (usecaseplanning.RecipeGenerator, error) {
	if cfg.LLMAPIKey == "" {
		slog.Warn("llm api key is not set, plan generation stays reuse-only")
		return nil, nil
	}
	provider, err := llmProvider(cfg.LLMProvider)
	if err != nil {
		return nil, err
	}
	client, err := llm.New(llm.Config{
		Provider: cfg.LLMProvider,
		Model:    cfg.LLMModel,
		Endpoint: cfg.LLMEndpoint,
		APIKey:   cfg.LLMAPIKey,
		Timeout:  cfg.LLMTimeout,
	}, llmrepo.New(store), provider)
	if err != nil {
		return nil, err
	}
	generator, err := llm.NewRecipeGenerator(client)
	if err != nil {
		return nil, err
	}
	return llm.NewPlanningGenerator(generator), nil
}

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
	plans := planningrepo.New(store)
	products := catalogrepo.New(store)
	recipes := reciperepo.New(store)
	generator, err := recipeGenerator(cfg, store)
	if err != nil {
		return err
	}

	handler := httpapi.New(httpapi.Deps{
		APIToken: cfg.APIToken,
		Catalog:  usecasecatalog.NewService(products, tx),
		Eater:    usecaseeater.NewService(eaters, tx, clock.System{}),
		Recipe:   usecaserecipe.NewService(recipes, tx, clock.System{}),
		Shopping: usecaseshopping.NewService(shoppingrepo.New(store), plans, products, tx),
		Reality: usecasereality.NewService(
			plans, feedbackrepo.New(store), analyticsrepo.New(store),
			products, recipes, eaters, tx, clock.System{},
		),
		Planning: usecaseplanning.NewService(
			plans, eaters, products, recipes, solver.New(), generator, tx,
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
