package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ZetoOfficial/meal-planner/internal/llm"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/repositories"
)

// App is the composition root for infrastructure adapters and use cases.
type App struct {
	Cfg          Config
	Pool         *pgxpool.Pool
	Repositories repositories.Repositories
	TxManager    *postgres.TxManager
	LLM          llm.Client
}

func New(ctx context.Context, cfg Config) (*App, error) {
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return &App{
		Cfg:          cfg,
		Pool:         pool,
		Repositories: repositories.New(pool),
		TxManager:    postgres.NewTxManager(pool),
		LLM:          llm.StubClient{Model: cfg.LLMModel},
	}, nil
}

func (a *App) Close() {
	if a.Pool != nil {
		a.Pool.Close()
	}
}
