# Layer Boundaries and Mockgen Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep `internal/app` configuration-only, make `cmd/api` the explicit composition root, and replace handwritten test doubles with pinned Uber mockgen output while unifying lint invocation.

**Architecture:** `cmd/api` owns the pool lifetime and wires Postgres adapters to the HTTP adapter. HTTP depends only on its `ReadinessChecker` consumer-side port; the Postgres adapter implements that port. Generated mocks replace all handwritten fake implementations in tests, while existing production stubs remain outside this change.

**Tech Stack:** Go 1.25, pgx v5, `go.uber.org/mock`, golangci-lint v2.12.2, Make, GitHub Actions.

---

### Task 1: Move composition root out of `internal/app`

**Files:**
- Delete: `internal/app/app.go`
- Create: `internal/api/http/readiness.go`
- Create: `internal/api/http/readiness_test.go`
- Create: `internal/api/http/mocks/readiness.go`
- Create: `internal/postgres/readiness.go`
- Modify: `cmd/api/main.go`
- Modify: `internal/api/http/server.go`
- Modify: `internal/api/http/server_test.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `Makefile`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `README.md`

- [ ] **Step 1: Generate the test-only readiness mock and write the failing HTTP test**

Add `go.uber.org/mock` v0.5.0 and define a lower-case `readinessChecker`
interface with `Check() error` in `internal/api/http/readiness_test.go`.
Configure `make mocks` to generate `internal/api/http/mocks/readiness.go` from
that test-only interface. Change the readiness test to require the generated
mock, assert that `Check` is invoked exactly once, and return an error from the
generated expectation. The test must fail because `Deps.Readiness` still has a
function type.

```go
checker.EXPECT().Check().Return(errors.New("database unavailable"))
handler := New(Deps{APIToken: "secret", Readiness: checker})
```

- [ ] **Step 2: Run mock generation and the focused test to verify RED**

Run: `make mocks`

Expected: the generated readiness mock is created.

Run: `docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine go test ./internal/api/http -run TestReadyzReturnsServiceUnavailableWhenReadinessFails -count=1`

Expected: FAIL because the generated mock is not assignable to the existing
`func() error` dependency.

- [ ] **Step 3: Implement the minimal layer boundary**

Add the HTTP-owned port and the Postgres implementation:

```go
package httpapi

type ReadinessChecker interface {
	Check() error
}
```

```go
package postgres

type Readiness struct { pool *pgxpool.Pool }

func NewReadiness(pool *pgxpool.Pool) *Readiness { return &Readiness{pool: pool} }

func (r *Readiness) Check() error { return r.pool.Ping(context.Background()) }
```

Replace `Deps.Readiness func() error` with `ReadinessChecker`; call
`d.Readiness.Check()` in `/readyz`. Delete `internal/app/app.go`. In
`cmd/api/main.go`, create the pool with `postgres.NewPool`, defer `pool.Close`,
build `postgres.NewReadiness(pool)`, and pass it to HTTP. Do not construct
repositories, TxManager, or an LLM client until a use case consumes them.
Update the architecture and README trees to describe `internal/app` as config
and `cmd/api` as the composition root.

- [ ] **Step 4: Run focused HTTP and full unit tests**

Run: `docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine go test ./internal/api/http -count=1`

Expected: PASS.

Run: `docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine go test ./...`

Expected: PASS.

### Task 2: Generate Uber mocks and remove handwritten test doubles

**Files:**
- Modify: `Makefile`
- Create: `internal/postgres/mocks/transaction.go`
- Create: `internal/postgres/mocks/pgx.go`
- Modify: `internal/api/http/server_test.go`
- Modify: `internal/postgres/tx_test.go`

- [ ] **Step 1: Extend mock generation and replace the transaction doubles**

Extend `make mocks`, which already invokes pinned
`go.uber.org/mock/mockgen` v0.6.0, to generate the Postgres transaction-starting
interface from its source file plus pgx `Tx`, `Row`, and sqlc `DBTX` mocks into
`internal/postgres/mocks`.

Rewrite transaction tests to create mocks via `gomock.NewController(t)`, set
ordered `EXPECT()` calls for Begin/Commit/Rollback/QueryRow/Scan, and remove
every handwritten `fakeTx`, `fakeRow`, and fake executor type.

- [ ] **Step 2: Run mock generation and verify generated files are current**

Run: `make mocks`

Expected: generated mock files exist and `git diff --exit-code -- internal/api/http/mocks internal/postgres/mocks` succeeds on a second run.

- [ ] **Step 3: Run the focused transaction tests**

Run: `docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine go test ./internal/postgres -count=1`

Expected: PASS with no handwritten fake/stub implementations.

- [ ] **Step 4: Enforce the test-double boundary**

Run: `rg -n -i '\\b(fake|stub)\\b' --glob '*_test.go' --glob '!**/mocks/**' .`

Expected: no handwritten fake or stub identifiers remain in tests. Keep
production `StubClient` and OpenAPI 501 handlers unchanged.

### Task 3: Make lint invocation identical locally and in CI

**Files:**
- Modify: `Makefile`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Make the local target use the pinned CI command**

Define and use the same command in Make:

```make
GOLANGCI := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

lint:
	$(GOLANGCI) run --timeout 5m
```

- [ ] **Step 2: Make CI delegate to the Make target**

Replace the lint workflow command with:

```yaml
run: make lint
```

- [ ] **Step 3: Verify lint locally and through the same target**

Run: `docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine make lint`

Expected: PASS using golangci-lint v2.12.2 with the five-minute timeout.

- [ ] **Step 4: Run complete verification**

Run: `docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine go test ./...`

Expected: PASS.

Run: `docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine go test -tags=integration -count=1 ./internal/postgres/...`

Expected: PASS with the intentional test-database skip when
`TEST_DATABASE_URL` is unset.

Run: `docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine go build ./...`

Expected: PASS.

- [ ] **Step 5: Commit the implementation**

```bash
git add cmd/api/main.go internal/app internal/api/http internal/postgres Makefile \
  .github/workflows/ci.yml go.mod go.sum docs/ARCHITECTURE.md README.md
git commit -m "refactor: clarify layer boundaries and mocks"
```
