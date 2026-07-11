//go:build integration

package postgrestest

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
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

	migrationSeparator = "---- create above / drop below ----"
	setupTimeout       = 30 * time.Second
)

type Database struct {
	Pool   *pgxpool.Pool
	Schema string
}

func RequireDatabase(t *testing.T) Database {
	t.Helper()

	dsn := os.Getenv(TestDatabaseURLEnv)
	if dsn == "" {
		t.Skipf("integration tests skipped: set %s to a disposable PostgreSQL database URL", TestDatabaseURLEnv)
	}

	ctx, cancel := context.WithTimeout(t.Context(), setupTimeout)
	defer cancel()

	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("find project root: %v", err)
	}

	controlPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to %s: %v", TestDatabaseURLEnv, err)
	}

	schema, err := testSchemaName()
	if err != nil {
		controlPool.Close()
		t.Fatalf("create test schema name: %v", err)
	}
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := controlPool.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		controlPool.Close()
		t.Fatalf("create isolated schema %s: %v", quotedSchema, err)
	}

	var testPool *pgxpool.Pool
	t.Cleanup(func() {
		if testPool != nil {
			testPool.Close()
		}

		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), setupTimeout)
		defer cleanupCancel()
		if _, err := controlPool.Exec(cleanupCtx, "DROP SCHEMA "+quotedSchema+" CASCADE"); err != nil {
			t.Errorf("drop isolated schema %s: %v", quotedSchema, err)
		}
		controlPool.Close()
	})

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse %s: %v", TestDatabaseURLEnv, err)
	}
	if poolConfig.ConnConfig.RuntimeParams == nil {
		poolConfig.ConnConfig.RuntimeParams = make(map[string]string)
	}

	poolConfig.ConnConfig.RuntimeParams["search_path"] = quotedSchema
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	testPool, err = pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("connect isolated test pool: %v", err)
	}
	if err := testPool.Ping(ctx); err != nil {
		t.Fatalf("ping isolated test pool: %v", err)
	}

	if err := applyMigrations(ctx, testPool, projectRoot); err != nil {
		t.Fatalf("apply migrations to isolated schema %q: %v", schema, err)
	}

	return Database{Pool: testPool, Schema: schema}
}

func testSchemaName() (string, error) {
	var randomBytes [4]byte
	if _, err := cryptorand.Read(randomBytes[:]); err != nil {
		return "", err
	}

	return fmt.Sprintf("meal_planner_test_%d_%d", time.Now().UTC().UnixNano(), binary.BigEndian.Uint32(randomBytes[:])), nil
}

func findProjectRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if isProjectRoot(directory) {
			return directory, nil
		}

		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("could not find project root from %q", directory)
		}
		directory = parent
	}
}

func isProjectRoot(directory string) bool {
	if _, err := os.Stat(filepath.Join(directory, "go.mod")); err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(directory, "migrations"))
	return err == nil && info.IsDir()
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool, projectRoot string) error {
	paths, err := filepath.Glob(filepath.Join(projectRoot, "migrations", "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return fmt.Errorf("no migrations found in %q", filepath.Join(projectRoot, "migrations"))
	}

	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		up, _, found := strings.Cut(string(contents), migrationSeparator)
		if !found {
			return fmt.Errorf("migration %s has no %q separator", path, migrationSeparator)
		}
		if _, err := pool.Exec(ctx, up); err != nil {
			return fmt.Errorf("apply %s: %w", filepath.Base(path), err)
		}
	}

	return nil
}
