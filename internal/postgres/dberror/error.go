package dberror

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

// Map translates a driver error for a read or a write. A foreign key violation
// there means the row being written points at a parent that does not exist.
func Map(err error, resource string) error {
	return mapError(err, resource, apperr.NotFound(resource))
}

// MapDelete translates a driver error for a delete. The same foreign key code
// means the opposite there: the row exists and other rows still reference it,
// which is a conflict, not a missing parent.
func MapDelete(err error, resource string) error {
	return mapError(err, resource, apperr.Conflict(resource+" is still in use"))
}

func mapError(err error, resource string, foreignKey error) error {
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
			return foreignKey
		}
	}
	return err
}
