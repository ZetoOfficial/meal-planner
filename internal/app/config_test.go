package app

import (
	"strings"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("API_TOKEN", "")
	t.Setenv("API_AUTH_DISABLED", "")
}

func TestLoadRejectsAnEmptyAPIToken(t *testing.T) {
	setRequiredEnv(t)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "API_TOKEN") {
		t.Fatalf("Load() error = %v, want an API_TOKEN error instead of an open API", err)
	}
}

func TestLoadAllowsAnEmptyTokenWhenAuthIsExplicitlyDisabled(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("API_AUTH_DISABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.AuthDisabled {
		t.Fatal("AuthDisabled = false, want true")
	}
}

func TestLoadKeepsTheConfiguredToken(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("API_TOKEN", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIToken != "secret" || cfg.AuthDisabled {
		t.Fatalf("cfg = {token %q, disabled %v}, want the token and auth on", cfg.APIToken, cfg.AuthDisabled)
	}
}

func TestDatabaseURLEscapesCredentials(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "meal_planner")
	t.Setenv("DB_USER", "owner")
	t.Setenv("DB_PASSWORD", "p@ss/w?rd")
	t.Setenv("DB_SSLMODE", "disable")

	got := databaseURL()
	want := "postgres://owner:p%40ss%2Fw%3Frd@localhost:5432/meal_planner?sslmode=disable"
	if got != want {
		t.Fatalf("databaseURL() = %q, want %q", got, want)
	}
}
