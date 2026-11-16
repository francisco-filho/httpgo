package assertion

import "regexp"

// MatchString reports whether a present string contains a regex match.
// Non-string values and absent paths never undergo coercion.
func MatchString(pattern *regexp.Regexp, actual any, present bool) bool {
	value, ok := actual.(string)
	return present && ok && pattern != nil && pattern.MatchString(value)
}
