// Package app — состав приложения (config + wiring контейнер).
package app

import (
	"fmt"
	"os"
	"time"
)

// Config — конфигурация сервиса (owner-only, без секрет-менеджера).
type Config struct {
	HTTPAddr     string        // адрес HTTP-сервера, напр. ":8080"
	DatabaseURL  string        // postgres connection string
	APIToken     string        // статический токен для X-API-Token (SPEC §9)
	LLMProvider  string        // "anthropic" | "openai"
	LLMModel     string        // напр. "claude-sonnet-4-6"
	LLMAPIKey    string        // ключ провайдера
	LLMTimeout   time.Duration // таймаут вызова LLM (SPEC §11: 30s)
	ShutdownWait time.Duration // grace-период на остановку
}

// Load читает конфиг из окружения. DatabaseURL обязателен.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:     env("HTTP_ADDR", ":8080"),
		DatabaseURL:  databaseURL(),
		APIToken:     os.Getenv("API_TOKEN"),
		LLMProvider:  env("LLM_PROVIDER", "anthropic"),
		LLMModel:     env("LLM_MODEL", "claude-sonnet-4-6"),
		LLMAPIKey:    os.Getenv("LLM_API_KEY"),
		LLMTimeout:   30 * time.Second,
		ShutdownWait: 10 * time.Second,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL (или DB_* переменные) обязателен")
	}
	return cfg, nil
}

// databaseURL берёт готовый DATABASE_URL либо собирает из DB_* переменных.
func databaseURL() string {
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	host := os.Getenv("DB_HOST")
	if host == "" {
		return ""
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"),
		host, env("DB_PORT", "5432"), os.Getenv("DB_NAME"),
		env("DB_SSLMODE", "disable"),
	)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
