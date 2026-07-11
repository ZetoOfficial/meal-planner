package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ZetoOfficial/meal-planner/internal/llm"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/repositories"
)

type App struct {
	Cfg          Config
	Repositories *repositories.Repositories
	TxManager    *postgres.TxManager
	LLM          llm.Client

	pool *pgxpool.Pool
}

func New(ctx context.Context, cfg Config) (*App, error) {
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return &App{
		Cfg:          cfg,
		Repositories: repositories.New(pool),
		TxManager:    postgres.NewTxManager(pool),
		LLM:          llm.StubClient{Model: cfg.LLMModel},
		pool:         pool,
	}, nil
}

func (a *App) Ping() error {
	return a.pool.Ping(context.Background())
}

func (a *App) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}
