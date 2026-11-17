package query

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// GO-325–328: node cardinality is not the type/cardinality of a JSON value.
func TestProjectionCardinality(t *testing.T) {
	for _, tc := range []struct {
		path, body, want string
		present          bool
	}{
		{"$[*]", `[]`, `null`, false},
		{"$[*]", `[null]`, `null`, true},
		{"$[*]", `[1]`, `1`, true},
		{"$[*]", `[1,2,1]`, `[1,2,1]`, true},
		{"$[*]", `[[]]`, `[]`, true},
		{"$[*]", `[[1],[],[[2]]]`, `[[1],[],[[2]]]`, true},
		{"$[*].x", `[{},{"x":null},{}]`, `null`, true},
		{"$[*].x", `[{},{"x":[]},{}]`, `[]`, true},
		{"$[*].x", `[{},{}]`, `null`, false},
		{"$[*][0]", `[[1,2],[],[3]]`, `[1,3]`, true},
		{"$[*][0][*]", `[[[1,2]]]`, `[1,2]`, true},
		{"$.categories[*].products[*].id", `{"categories":[{"products":[{"id":3},{},{"id":1}]},{},{"products":[]},{"products":[{"id":3}]}]}`, `[3,1,3]`, true},
		{"$[0][*].n", `[[{"n":9007199254740993},{"n":1.00},{"n":1e999999999999}]]`, `[9007199254740993,1.00,1e999999999999]`, true},
	} {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			p, err := Parse(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := Decode([]byte(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(doc)
			for range 2 {
				value, present, err := p.SelectValue(doc)
				encoded, _ := json.Marshal(value)
				if err != nil || present != tc.present || string(encoded) != tc.want {
					t.Fatalf("value=%s present=%v err=%v want=%s,%v", encoded, present, err, tc.want, tc.present)
				}
				selected, found, err := p.Select([]byte(tc.body))
				if err != nil || found != present || !reflect.DeepEqual(selected, value) {
					t.Fatalf("capture=%v,%v,%v", selected, found, err)
				}
			}
			after, _ := json.Marshal(doc)
			if string(before) != string(after) {
				t.Fatal("document mutated")
			}
		})
	}
}

func TestProjectionStrictTraversal(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"$[*]", `{}`}, {"$[*]", `1`}, {"$[*]", `null`},
		{"$.a[*]", `1`}, {"$.a[0][*]", `{"a":{}}`},
		{"$[*].a", `[{"a":1},null]`}, {"$[*].a", `[null,{"a":1}]`},
		{"$[*][0]", `[[1],{}]`}, {"$[*][*]", `[[1],null]`},
	} {
		p, err := Parse(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		value, present, err := p.Select([]byte(tc.body))
		if err == nil || value != nil || present || !strings.Contains(err.Error(), "percurso") {
			t.Fatalf("%+v: %v,%v,%v", tc, value, present, err)
		}
	}
	for _, body := range []string{"", "[", "[] null"} {
		p, _ := Parse("$[*]")
		if _, _, err := p.Select([]byte(body)); err == nil {
			t.Fatal("invalid JSON approved")
		}
	}
}

func TestProjectionMalformedPaths(t *testing.T) {
	for _, source := range []string{"$[*", "$[**]", "$[*0]", "$[*:]", "$.*", "$..x", "$[0:2]", "$[0,1]", "$[?(@.x)]"} {
		if _, err := Parse(source); err == nil {
			t.Fatalf("accepted %s", source)
		}
	}
}
