package app

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	defaultLLMTimeout   = 30 * time.Second
	defaultShutdownWait = 10 * time.Second
)

type Config struct {
	HTTPAddr     string
	DatabaseURL  string
	APIToken     string
	LLMProvider  string
	LLMModel     string
	LLMAPIKey    string
	LLMTimeout   time.Duration
	ShutdownWait time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:     env("HTTP_ADDR", ":8080"),
		DatabaseURL:  databaseURL(),
		APIToken:     os.Getenv("API_TOKEN"),
		LLMProvider:  env("LLM_PROVIDER", "anthropic"),
		LLMModel:     env("LLM_MODEL", "claude-sonnet-4-6"),
		LLMAPIKey:    os.Getenv("LLM_API_KEY"),
		LLMTimeout:   defaultLLMTimeout,
		ShutdownWait: defaultShutdownWait,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("config: DATABASE_URL (или DB_* переменные) обязателен")
	}
	return cfg, nil
}

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
