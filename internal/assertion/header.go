package assertion

import "strings"

type HeaderOperator string

const (
	HeaderEqual    HeaderOperator = "=="
	HeaderContains HeaderOperator = "contains"
	HeaderExists   HeaderOperator = "exists"
)

// Header compares a present value literally. Missing is never an empty value.
func Header(operator HeaderOperator, expected, actual string, present bool) bool {
	if !present {
		return false
	}
	switch operator {
	case HeaderEqual:
		return actual == expected
	case HeaderContains:
		return strings.Contains(actual, expected)
	case HeaderExists:
		return true
	default:
		return false
	}
}
