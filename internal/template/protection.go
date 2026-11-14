package template

import (
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// SecretVariants enumerates known raw, decoded, JSON/Go-escaped and URL-escaped
// forms, even before context validation. It is not arbitrary encoding detection.
// Composite values remain opaque to resolution; walking here only protects their
// constituents from appearing in diagnostics such as a quoted filename.
func SecretVariants(source string) []string {
	v := parseVariable(source)
	bases := []string{source, string(v.raw), v.text}
	if v.composite {
		decoder := json.NewDecoder(strings.NewReader(string(v.raw)))
		decoder.UseNumber()
		var decoded any
		if decoder.Decode(&decoded) == nil {
			bases = append(bases, secretParts(decoded)...)
		}
	}
	var variants []string
	for _, base := range bases {
		if base == "" {
			continue
		}
		encoded, _ := json.Marshal(base)
		quoted := strconv.Quote(base)
		variants = append(variants, base, string(encoded[1:len(encoded)-1]), quoted[1:len(quoted)-1], url.PathEscape(base), url.QueryEscape(base))
	}
	return variants
}

func secretParts(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case json.Number:
		return []string{v.String()}
	case bool:
		return []string{strconv.FormatBool(v)}
	case nil:
		return []string{"null"}
	case []any:
		var parts []string
		for _, item := range v {
			parts = append(parts, secretParts(item)...)
		}
		return parts
	case map[string]any:
		var parts []string
		for key, item := range v {
			parts = append(parts, key)
			parts = append(parts, secretParts(item)...)
		}
		return parts
	default:
		return nil
	}
}

// Protector replaces longest matches once, without rewriting the redaction marker.
func Protector(values []string) func(string) string {
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	pairs := make([]string, 0, 2*len(values))
	for _, value := range values {
		if value != "" {
			pairs = append(pairs, value, "[oculto]")
		}
	}
	return strings.NewReplacer(pairs...).Replace
}
