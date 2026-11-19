package query

import (
	"encoding/json"
	"errors"
	"testing"
)

// GO-345–346: equality is typed, exact and independent of the jp number parser.
func TestFilterTypedEquality(t *testing.T) {
	for _, tc := range []struct {
		literal, value string
		matches        bool
	}{
		{`'pizza'`, `"pizza"`, true}, {`"pizza"`, `"juice"`, false},
		{`true`, `true`, true}, {`false`, `false`, true}, {`true`, `1`, false},
		{`null`, `null`, true}, {`null`, `[]`, false}, {`1`, `"1"`, false},
		{`1`, `{}`, false}, {`9007199254740993`, `9007199254740992`, false},
		{`9223372036854775809`, `9223372036854775809`, true},
		{`1.0000000000000000000002`, `1.0000000000000000000001`, false},
		{`1.0000000000000000000002`, `1.0000000000000000000002`, true},
		{`1.00`, `1e0`, true}, {`-0`, `0e99999999999999999999999`, true},
		{`1e99999999999999999999999`, `10e99999999999999999999998`, true},
		{`1e-99999999999999999999999`, `0`, false},
		{`'it\'s "pizza"'`, `"it's \"pizza\""`, true},
		{`"a\"b/c\\d"`, `"a\"b/c\\d"`, true},
		{`'\uD83C\uDF55\n\t'`, `"🍕\n\t"`, true},
		{`'x)].id[?(@.a==1)'`, `"x)].id[?(@.a==1)"`, true},
	} {
		t.Run(tc.literal+tc.value, func(t *testing.T) {
			source := "$[?(@.v == " + tc.literal + ")].id"
			p, err := Parse(source)
			if err != nil || !p.Expansive() {
				t.Fatalf("parse %s: %v", source, err)
			}
			value, present, err := p.Select([]byte(`[{"v":` + tc.value + `,"id":9007199254740993}]`))
			if err != nil || present != tc.matches || present && value != json.Number("9007199254740993") {
				t.Fatalf("value=%v present=%v err=%v", value, present, err)
			}
			if got := p.RedactLiterals(source); got != "$[?(@.v == [literal oculto])].id" {
				t.Fatalf("display=%s", got)
			}
		})
	}
	p, _ := Parse("$[?(@.v == null)].id")
	if got, present, err := p.Select([]byte(`[{"id":1}]`)); got != nil || present || err != nil {
		t.Fatalf("missing matched null: %v %v %v", got, present, err)
	}
}

// GO-352: excluded syntax cannot hitchhike on the library's wider dialect.
func TestFilterValidation(t *testing.T) {
	for _, tc := range []struct {
		source      string
		unsupported bool
	}{
		{`$[?(@.x)]`, true}, {`$[?(@.x != 1)]`, true}, {`$[?(@.x > 1)]`, true},
		{`$[?(@.x == 1 && @.y == 2)]`, true}, {`$[?(@.x == 1 || @.y == 2)]`, true},
		{`$[?(@.x =~ 'a')]`, true}, {`$[?(@.x == [])]`, true}, {`$[?(@.x == {})]`, true},
		{`$[?(@ == 1)]`, true}, {`$[?(@['x'] == 1)]`, true}, {`$[?(@.x[*] == 1)]`, true},
		{`$[?(@.x[-1] == 1)]`, true}, {`$[?(@.x() == 1)]`, true},
		{`$[?(@.x == {{value}})]`, true}, {`$[?(@.{{field}} == 1)]`, true},
		{`$[?(@.x == 'unterminated)]`, false}, {`$[?(@.x == '\q')]`, false},
		{`$[?(@.x == 1)`, false}, {`$[?(@.x == )]`, false}, {`$[?(@.x == 01)]`, false},
	} {
		t.Run(tc.source, func(t *testing.T) {
			_, err := Parse(tc.source)
			var located *Error
			if !errors.As(err, &located) || located.Unsupported != tc.unsupported {
				t.Fatalf("%s: %#v", tc.source, err)
			}
		})
	}
}

// GO-355: redact every operand, inspect decoded values and preserve simple paths.
func TestFilterLiteralPresentation(t *testing.T) {
	source := `$[?(@.x=='private')].items[?(@.id==9223372036854775809)].v`
	p, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.RedactLiterals(source); got != `$[?(@.x==[literal oculto])].items[?(@.id==[literal oculto])].v` {
		t.Fatalf("display=%s", got)
	}
	if got := p.RedactLiterals(""); got != "[consulta com filtro]" {
		t.Fatalf("mismatched source display=%s", got)
	}
	if !p.HasProtectedLiteral(func(value any) bool { return value == json.Number("9223372036854775809") }) {
		t.Fatal("second decoded operand not inspected")
	}
	if p.HasProtectedLiteral(func(any) bool { return false }) {
		t.Fatal("unprotected literal classified protected")
	}
	simple, _ := Parse("$.x")
	if got := simple.RedactLiterals("$.x"); got != "$.x" {
		t.Fatalf("simple display=%s", got)
	}
}

// GO-357–358: incompatible branches invalidate the entire selection in both APIs.
func TestFilterStrictEvaluation(t *testing.T) {
	for _, tc := range []struct {
		path, body string
		fails      bool
	}{
		{`$[?(@.x == null)]`, `null`, true},
		{`$[?(@.x == null)]`, `[{"x":null},null]`, true},
		{`$[?(@.x.y == 1)]`, `[{"x":{"y":1}},{"x":null}]`, true},
		{`$[?(@.x[0] == 1)]`, `[{"x":[1]},{"x":{}}]`, true},
		{`$[*][?(@.x == 1)]`, `[[{"x":1}],{}]`, true},
		{`$[?(@.x.y == 1)]`, `[{"x":{"y":1}},{}]`, false},
		{`$[?(@.x[0] == 1)]`, `[{"x":[1]},{"x":[]}]`, false},
		{`$[?(@.x == 1)]`, ``, true}, {`$[?(@.x == 1)]`, `[] []`, true},
		{`$[?(@.x == 1)]`, `[{`, true},
	} {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			p, err := Parse(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			value, present, err := p.Select([]byte(tc.body))
			if (err != nil) != tc.fails || tc.fails && (value != nil || present) {
				t.Fatalf("value=%v present=%v err=%v", value, present, err)
			}
		})
	}
}
