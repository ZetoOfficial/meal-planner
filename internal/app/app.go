package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ZetoOfficial/meal-planner/internal/llm"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
)

// App — корневой контейнер зависимостей (composition root).
// Здесь собираются pool, репозитории, use cases, solver, quality, llm и
// передаются в HTTP-слой. Пока — минимальный каркас (ARCHITECTURE §4).
type App struct {
	Cfg  Config
	Pool *pgxpool.Pool
	LLM  llm.Client
}

// New поднимает зависимости приложения по конфигу.
func New(ctx context.Context, cfg Config) (*App, error) {
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return &App{
		Cfg:  cfg,
		Pool: pool,
		LLM:  llm.StubClient{Model: cfg.LLMModel}, // TODO: реальный провайдер
	}, nil
}

// Close освобождает ресурсы.
func (a *App) Close() {
	if a.Pool != nil {
		a.Pool.Close()
	}
}
