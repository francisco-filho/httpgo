package query

import "strings"

// HeaderValue preserves each occurrence, including empty values, in wire order.
type HeaderValue struct{ Name, Value string }

// Header returns only the first occurrence; commas are not separators.
func Header(headers []HeaderValue, name string) (string, bool) {
	for _, header := range headers {
		if strings.EqualFold(header.Name, name) {
			return header.Value, true
		}
	}
	return "", false
}
