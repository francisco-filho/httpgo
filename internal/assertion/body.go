package assertion

import "strings"

// BodyOperator is a literal substring predicate on decoded response text.
type BodyOperator string

const (
	BodyContains    BodyOperator = "contains"
	BodyNotContains BodyOperator = "not contains"
)

func Body(operator BodyOperator, expected, actual string) bool {
	switch operator {
	case BodyContains:
		return strings.Contains(actual, expected)
	case BodyNotContains:
		return !strings.Contains(actual, expected)
	default:
		return false
	}
}
