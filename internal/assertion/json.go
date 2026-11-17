package assertion

import (
	"encoding/json"

	"capimgrosso.com/httpgo/internal/query"
)

// JSONEqual compares decoded JSON values structurally, without coercion.
func JSONEqual(expected, actual any) bool { return query.JSONEqual(expected, actual) }

func compareNumber(a, b json.Number) int { return query.CompareNumber(a, b) }

// ValidCount accepts integral nonnegative JSON numbers without a machine limit.
func ValidCount(value any) bool { return query.ValidCount(value) }
