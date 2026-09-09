package pgvalue

import "github.com/jackc/pgx/v5/pgtype"

// NumericFields converts a batch of nullable numerics at once. A NULL column
// reads as zero: the summary tables use NULL for "nothing was recorded", and
// the coverage score, not the value, is what says how much to trust that.
func NumericFields(fields map[string]pgtype.Numeric) (map[string]float64, error) {
	result := make(map[string]float64, len(fields))
	for name, value := range fields {
		if !value.Valid {
			result[name] = 0
			continue
		}
		converted, err := Float64(value)
		if err != nil {
			return nil, err
		}
		result[name] = converted
	}
	return result, nil
}
