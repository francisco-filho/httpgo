package query

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSelect(t *testing.T) {
	for _, tc := range []struct {
		path, body string
		want       any
		found      bool
	}{
		{"$", `"root"`, "root", true},
		{"$.product.items[0].token", `{"product":{"items":[{"token":""}]}}`, "", true},
		{"$[1][0]", `[[],["token"]]`, "token", true},
		{"$.missing", `{"other":null}`, nil, false},
		{"$.other", `{"other":null}`, nil, true},
		{"$[1]", `["x"]`, nil, false},
		{"$.a.b", `{"a":1}`, nil, false},
		{"$", `900719925474099312345`, json.Number("900719925474099312345"), true},
		{"$", `true`, true, true},
	} {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			p, err := Parse(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			got, found, err := p.Select([]byte(tc.body))
			if err != nil || found != tc.found || got != tc.want {
				t.Fatalf("got %v,%v,%v want %v,%v", got, found, err, tc.want, tc.found)
			}
		})
	}
	p, _ := Parse("$")
	for _, body := range []string{"", `{"secret":`, `"secret" null`, "NaN"} {
		if _, _, err := p.Select([]byte(body)); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("unsafe/absent error: %v", err)
		}
	}
	for _, body := range []string{`[]`, `{}`} {
		if _, found, err := p.Select([]byte(body)); !found || err != nil {
			t.Fatalf("%s: %v,%v", body, found, err)
		}
	}
}

func TestInvalidPaths(t *testing.T) {
	for _, source := range []string{"", "x", "$.", "$[", "$[]", "$[-1]", "$.*", "$..token", "$[?(@.token)]", "$['token']", "$[0:1]", "$[0,1]", "$[99999999999999999999999999999999]", "$.x trailing", "$foo"} {
		if _, err := Parse(source); err == nil || strings.Contains(err.Error(), source) && source != "" && source != "x" {
			t.Fatalf("Parse(%q)=%v", source, err)
		}
	}
}
