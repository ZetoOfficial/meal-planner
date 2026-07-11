# M1 Infrastructure Design

## Scope

Complete the remaining infrastructure-only work in M1:

- transaction and query-executor wiring for Postgres;
- composition-root construction of infrastructure dependencies;
- repeatable, opt-in Postgres integration-test harness;
- developer commands and documentation for the harness.

OpenAPI endpoints, CRUD use cases, domain behavior, and every form of
`not implemented` repository method are out of scope. Existing M1 OpenAPI
wiring and shared application-error mapping are already present and are not
changed by this work.

## Transaction and repository wiring

`postgres.TxManager` owns a `pgx` transaction boundary. Its callback receives
only a context, keeping `pgx`, sqlc, and Postgres adapters out of use cases.
Repository constructors accept the generated sqlc `DBTX` executor, so the
composition root can construct a pool-backed repository set and the
transaction manager can construct an equivalent set backed by a `pgx.Tx`.

The repository set is infrastructure wiring only. It exposes typed executor
holders for future repository implementations but does not claim to implement
use-case ports or add placeholder methods. A future use case will receive its
own consumer-side port and a fully implemented adapter when that behavior is
in scope.

The transaction manager must roll back when the callback returns an error,
commit when it returns nil, and return begin/commit errors unchanged. Unit
tests use a minimal fake `pgx.Tx` solely to observe those transaction effects.

## Integration test harness

Integration tests are compiled only with the `integration` build tag and run
only when `TEST_DATABASE_URL` is set. The harness creates a unique schema in
that database, connects a test pool with that schema as its only explicit
`search_path`,
applies every up migration from `migrations/`, and drops the schema during
cleanup. An unset URL produces a clear skip rather than a failure.

The first integration test proves that migrations are applied to the isolated
schema. It does not test application CRUD or a complete product flow.

## Developer workflow

`make test` runs unit tests and reports whether integration tests were skipped
or run. `make test-integration` runs the tagged Postgres tests and documents
its `TEST_DATABASE_URL` prerequisite. `.env.example` and the README describe
the test URL and local command sequence.

## Verification

The implementation follows test-first development: transaction behavior tests
must fail before production code is added; the integration test/harness is
added in a separate task. Final verification runs unit tests, tagged
integration tests when a local test database is available, the build, and the
repository status check. The current local Go 1.20.5 toolchain cannot compile
this Go 1.25 project, so full local Go verification requires the project
toolchain or CI.
