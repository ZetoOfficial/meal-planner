package dberror_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/dberror"
)

func TestMapReportsAMissingParentRowAsNotFound(t *testing.T) {
	err := dberror.Map(&pgconn.PgError{Code: "23503"}, "product")

	assertCode(t, err, apperr.CodeNotFound)
}

// Deleting a product a plan still uses is a conflict: the product exists, it is
// just in use. Reporting "product not found" would be a lie.
func TestMapDeleteReportsAStillReferencedRowAsConflict(t *testing.T) {
	err := dberror.MapDelete(&pgconn.PgError{Code: "23503"}, "product")

	assertCode(t, err, apperr.CodeConflict)
}

func TestMapDeleteStillReportsDuplicatesAsConflict(t *testing.T) {
	err := dberror.MapDelete(&pgconn.PgError{Code: "23505"}, "product")

	assertCode(t, err, apperr.CodeConflict)
}

func assertCode(t *testing.T, err error, want apperr.Code) {
	t.Helper()
	var appError *apperr.Error
	if !errors.As(err, &appError) {
		t.Fatalf("error = %v, want an application error", err)
	}
	if appError.Code != want {
		t.Fatalf("code = %q, want %q", appError.Code, want)
	}
}
