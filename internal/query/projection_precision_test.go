package query

import (
	"encoding/json"
	"reflect"
	"testing"
)

// GO-446–448: navigation must preserve json.Number and its original spelling.
func TestProjectionExactNumbers(t *testing.T) {
	const numbers = `[9007199254740992,9007199254740993,9223372036854775808,9223372036854775809,1.0000000000000000000001,1.0000000000000000000002,1.00,1e0,1e99999999999999999999999,1e-99999999999999999999999]`
	doc, err := Decode([]byte(`{"_0":{"01":[` + numbers + `]}}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"$._0.01[0]", "$._0.01[*][*]"} {
		p, err := Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			got, present, err := p.SelectValue(doc)
			encoded, marshalErr := json.Marshal(got)
			if err != nil || marshalErr != nil || !present || string(encoded) != numbers {
				t.Fatalf("%s: got=%s present=%v err=%v marshal=%v", source, encoded, present, err, marshalErr)
			}
			for _, value := range got.([]any) {
				if _, ok := value.(json.Number); !ok {
					t.Fatalf("%s: number became %T", source, value)
				}
			}
		}
	}
}

func TestProjectionCompiledSelectionIsStatic(t *testing.T) {
	for _, source := range []string{"$", "$.a[0]._1", "$.a[*]._1"} {
		p, err := Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		if p.Expansive() != (source == "$.a[*]._1") {
			t.Fatalf("wrong selection for %s", source)
		}
		if p.Expansive() && len(p.projection) != len(p.steps) {
			t.Fatal("incomplete projection plan")
		}
	}
	p, _ := Parse("$.a.b")
	if value, present, err := p.SelectValue(map[string]any{"a": nil}); value != nil || present || err == nil {
		t.Fatal("simple evaluation no longer strict")
	}
	if value, present, err := p.Select([]byte(`{"a":null}`)); value != nil || present || err != nil {
		t.Fatal("legacy capture absence changed")
	}
	if !reflect.DeepEqual(Path{}, func() Path { p, _ := Parse("$"); return p }()) {
		t.Fatal("root plan changed")
	}
}
