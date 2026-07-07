package postgrestest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	TestDatabaseURLEnv = "TEST_DATABASE_URL"
	setupTimeout       = 30 * time.Second
	cleanupTimeout     = 10 * time.Second
)

type Database struct {
	Pool   *pgxpool.Pool
	Schema string
}

func RequireDatabase(t *testing.T) Database {
	t.Helper()

	dsn := os.Getenv(TestDatabaseURLEnv)
	if dsn == "" {
		t.Skipf("set %s to run Postgres integration tests", TestDatabaseURLEnv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), setupTimeout)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}

	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		adminPool.Close()
		t.Fatalf("create isolated test schema: %v", err)
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		cleanupSchema(adminPool, schema)
		adminPool.Close()
		t.Fatalf("parse test database dsn: %v", err)
	}
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = make(map[string]string)
	}
	config.ConnConfig.RuntimeParams["search_path"] = pgx.Identifier{schema}.Sanitize() + ",public"

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		cleanupSchema(adminPool, schema)
		adminPool.Close()
		t.Fatalf("connect isolated test schema: %v", err)
	}

	if err := applyMigrations(ctx, pool, schema); err != nil {
		pool.Close()
		cleanupSchema(adminPool, schema)
		adminPool.Close()
		t.Fatalf("apply migrations to isolated test schema: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		cleanupSchema(adminPool, schema)
		adminPool.Close()
	})

	return Database{Pool: pool, Schema: schema}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(root, "migrations", "*.sql"))
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(files)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+pgx.Identifier{schema}.Sanitize()+", public"); err != nil {
		return fmt.Errorf("set migration search_path: %w", err)
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filepath.Base(file), err)
		}
		upSQL := strings.SplitN(string(content), "---- create above / drop below ----", 2)[0]
		if strings.TrimSpace(upSQL) == "" {
			continue
		}
		if _, err := tx.Exec(ctx, upSQL); err != nil {
			return fmt.Errorf("execute migration %s: %w", filepath.Base(file), err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "migrations")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find project root containing migrations")
		}
		dir = parent
	}
}

func cleanupSchema(pool *pgxpool.Pool, schema string) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
}
