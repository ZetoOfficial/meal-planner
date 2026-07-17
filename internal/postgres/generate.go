package postgres

//go:generate go tool mockgen -destination=mocks/pgx.go -package=mocks github.com/jackc/pgx/v5 Tx,Row
//go:generate go tool mockgen -source=sqlc/db.go -imports=pgx=github.com/jackc/pgx/v5,pgconn=github.com/jackc/pgx/v5/pgconn -destination=mocks/dbtx.go -package=mocks
