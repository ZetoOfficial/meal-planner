# M1 Infrastructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish M1's remaining Postgres transaction foundation and repeatable integration-test infrastructure without implementing CRUD or adding placeholders.

**Architecture:** Keep Postgres/sqlc types inside the adapter. A transaction manager begins, commits, or rolls back `pgx.Tx`; a repository wiring set can be constructed from either pool or transaction executors without pretending to satisfy unimplemented use-case ports. Tagged integration tests create one isolated schema, apply migrations, and clean it up.

**Tech Stack:** Go 1.25, pgx v5, sqlc-generated queries, standard library testing, PostgreSQL, Make.

---

### Task 1: Transaction runner and executor wiring

**Files:**
- Create: `internal/postgres/repositories/repositories.go`
- Modify: `internal/postgres/tx.go`
- Modify: `internal/app/app.go`
- Test: `internal/postgres/tx_test.go`

- [ ] **Step 1: Write failing transaction tests**

Create `internal/postgres/tx_test.go` with a minimal `pgx.Tx` fake that records
`Commit`, `Rollback`, and `QueryRow`. Add one test that calls `WithTx` with nil
error and expects exactly one commit and no rollback, and one test that returns
`errors.New("callback failed")` and expects one rollback and no commit. Add a
test that creates `repositories.New(fakeTx)` and calls its sqlc query accessor
to prove the transaction is accepted as the executor.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./internal/postgres -run 'TestTxManager|TestRepositories' -count=1`

Expected: compilation or assertion failure because the repository wiring set
and required transaction behavior do not exist yet.

- [ ] **Step 3: Implement the minimal adapter-only wiring**

Create `repositories.New(db sqlc.DBTX) *Repositories`, where `Repositories`
owns `*sqlc.Queries` and provides a `Queries() *sqlc.Queries` accessor. Do not
add repository methods, errors, or use-case-port assertions. Update
`TxManager` to depend on a narrow `Begin(context.Context) (pgx.Tx, error)`
interface for testing, call the callback once, rollback on callback failure,
and commit otherwise. Update `app.App` to construct and retain the pool-backed
repository wiring and `TxManager` in the composition root.

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run: `go test ./internal/postgres -run 'TestTxManager|TestRepositories' -count=1`

Expected: PASS; the commit test sees one commit, the callback-error test sees
one rollback, and the repository wiring test observes its transaction-backed
executor.

- [ ] **Step 5: Run the unit suite and commit**

Run: `go test ./...`

Expected: PASS with Go 1.25 or newer. Commit the focused changes as
`feat(postgres): complete transaction foundation`.

### Task 2: Isolated Postgres integration-test harness

**Files:**
- Create: `internal/testkit/postgres/database.go`
- Create: `internal/postgres/integration_test.go`
- Modify: `Makefile`
- Modify: `.env.example`
- Modify: `README.md`
- Test: `internal/postgres/integration_test.go`

- [ ] **Step 1: Write the failing tagged integration test**

Create `internal/postgres/integration_test.go` with `//go:build integration`.
Call `testpostgres.RequireDatabase(t)`, then query `information_schema.tables`
for the `product` table in `db.Schema`; fail if it does not exist. This test
must use `TEST_DATABASE_URL` rather than the development database URL.

- [ ] **Step 2: Run the tagged test and verify RED**

Run: `TEST_DATABASE_URL="$TEST_DATABASE_URL" go test -tags=integration -count=1 -v ./internal/postgres/...`

Expected: compilation failure because `testpostgres.RequireDatabase` does not
exist, or a clear skip if `TEST_DATABASE_URL` is unset.

- [ ] **Step 3: Implement the isolated-schema harness**

Create `testpostgres.RequireDatabase(t)` to read `TEST_DATABASE_URL`, skip
with an actionable message if it is unset, create a schema named from a unique
timestamp, configure a pool `search_path` as the isolated `<schema>` only, apply the up
section of every sorted `migrations/*.sql` file inside that schema, and register
cleanup that closes pools and drops the schema with `CASCADE`. Fail setup with
`t.Fatalf`; never fall back to the development schema.

- [ ] **Step 4: Add repeatable commands and documentation**

Update `Makefile` so `test-unit` runs `go test ./...`, `test-integration` runs
`go test -tags=integration -count=1 -v ./internal/postgres/...`, and `test`
runs unit tests then either integration tests (when `TEST_DATABASE_URL` is
set) or prints `integration tests skipped: set TEST_DATABASE_URL to run make
test-integration`. Add an opt-in test URL to `.env.example` and document the
same environment variable and commands in `README.md`.

- [ ] **Step 5: Run the integration test and verify GREEN**

Run: `TEST_DATABASE_URL="$TEST_DATABASE_URL" go test -tags=integration -count=1 -v ./internal/postgres/...`

Expected: PASS when a reachable Postgres database is configured; output proves
migrations created `product` in a disposable schema. With no URL, report the
intentional skip without treating it as a passing integration run.

- [ ] **Step 6: Run the full verification set and commit**

Run: `make test && make build`

Expected: unit suite and build pass with Go 1.25 or newer; `make test` reports
whether integration tests ran or were skipped. Commit as
`test(postgres): add integration test harness`.
