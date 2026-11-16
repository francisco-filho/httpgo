package query

import (
	"fmt"
	"regexp"
)

// Regex extracts only the first group of the first match. Indices distinguish
// an empty participating group from an optional group that did not participate.
// The caller owns compilation; diagnostics never retain the input or pattern.
func Regex(compiled *regexp.Regexp, text string) (string, error) {
	if compiled == nil || compiled.NumSubexp() == 0 {
		return "", fmt.Errorf("regex de captura exige primeiro grupo")
	}
	indices := compiled.FindStringSubmatchIndex(text)
	if indices == nil {
		return "", fmt.Errorf("regex de captura não encontrou correspondência")
	}
	if indices[2] < 0 {
		return "", fmt.Errorf("primeiro grupo da regex não participou da correspondência")
	}
	return text[indices[2]:indices[3]], nil
}
