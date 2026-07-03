package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ZetoOfficial/meal-planner/internal/llm"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/repositories"
	cataloguc "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
)

// App is the composition root for infrastructure adapters and use cases.
type App struct {
	Cfg          Config
	Repositories repositories.Repositories
	Catalog      *cataloguc.Service
	LLM          llm.Client
	pool         *pgxpool.Pool
}

func New(ctx context.Context, cfg Config) (*App, error) {
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	repos := repositories.New(pool)
	return &App{
		Cfg:          cfg,
		Repositories: repos,
		Catalog:      cataloguc.NewService(repos.Products),
		LLM:          llm.StubClient{Model: cfg.LLMModel},
		pool:         pool,
	}, nil
}

func (a *App) Ping(ctx context.Context) error {
	if a.pool == nil {
		return nil
	}
	return a.pool.Ping(ctx)
}

func (a *App) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}
