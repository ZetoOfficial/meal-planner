package pgvalue

import (
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
)

func Numeric(value float64) pgtype.Numeric {
	var number pgtype.Numeric
	if err := number.Scan(strconv.FormatFloat(value, 'f', -1, 64)); err != nil {
		panic(fmt.Sprintf("convert numeric: %v", err))
	}
	return number
}

func OptionalNumeric(value *float64) pgtype.Numeric {
	if value == nil {
		return pgtype.Numeric{}
	}
	return Numeric(*value)
}

func Float64(value pgtype.Numeric) (float64, error) {
	converted, err := value.Float64Value()
	if err != nil {
		return 0, fmt.Errorf("convert numeric to float64: %w", err)
	}
	return converted.Float64, nil
}

func OptionalFloat64(value pgtype.Numeric) (*float64, error) {
	if !value.Valid {
		return nil, nil
	}
	converted, err := Float64(value)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}
