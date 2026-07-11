package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ZetoOfficial/meal-planner/internal/llm"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/repositories"
)

// App — корневой контейнер зависимостей (composition root).
// Здесь собираются pool, репозитории, use cases, solver, quality, llm и
// передаются в HTTP-слой. Пока — минимальный каркас (ARCHITECTURE §4).
type App struct {
	Cfg          Config
	Repositories *repositories.Repositories
	TxManager    *postgres.TxManager
	LLM          llm.Client

	pool *pgxpool.Pool
}

// New поднимает зависимости приложения по конфигу.
func New(ctx context.Context, cfg Config) (*App, error) {
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return &App{
		Cfg:          cfg,
		Repositories: repositories.New(pool),
		TxManager:    postgres.NewTxManager(pool),
		LLM:          llm.StubClient{Model: cfg.LLMModel}, // TODO: реальный провайдер
		pool:         pool,
	}, nil
}

// Ping reports whether the application's database connection is ready.
func (a *App) Ping(ctx context.Context) error {
	return a.pool.Ping(ctx)
}

// Close освобождает ресурсы.
func (a *App) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}
