package assertion

import "strings"

type HeaderOperator string

const (
	HeaderEqual       HeaderOperator = "=="
	HeaderContains    HeaderOperator = "contains"
	HeaderNotContains HeaderOperator = "not contains"
	HeaderExists      HeaderOperator = "exists"
)

// Header compares a value literally. Only not contains approves absence.
func Header(operator HeaderOperator, expected, actual string, present bool) bool {
	if !present {
		return operator == HeaderNotContains
	}
	switch operator {
	case HeaderEqual:
		return actual == expected
	case HeaderContains:
		return strings.Contains(actual, expected)
	case HeaderNotContains:
		return !strings.Contains(actual, expected)
	case HeaderExists:
		return true
	default:
		return false
	}
}
