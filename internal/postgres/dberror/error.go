package dberror

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

func Map(err error, resource string) error {
	var appError *apperr.Error
	if err == nil || errors.As(err, &appError) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound(resource)
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		switch databaseError.Code {
		case "23505":
			return apperr.Conflict(resource + " already exists")
		case "23503":
			return apperr.NotFound(resource)
		}
	}
	return err
}
