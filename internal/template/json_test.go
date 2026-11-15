package template

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONTypedValues(t *testing.T) {
	for _, tt := range []struct{ value, want string }{
		{`"001"`, `"001"`}, {"Pizza", `"Pizza"`}, {"", `""`}, {"null", "null"},
		{"false", "false"}, {"9007199254740993123456789", "9007199254740993123456789"},
		{`[1,{"x":9007199254740993123456789}]`, `[1,{"x":9007199254740993123456789}]`},
		{`{"x":"{{missing}}"}`, `{"x":"{{missing}}"}`},
	} {
		t.Run(tt.value, func(t *testing.T) {
			c, err := New([]string{"x=" + tt.value}, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, wrapper := range [][2]string{{"", ""}, {`{"a":`, `}`}, {`[`, `]`}} {
				got, err := c.RenderJSON([]byte(wrapper[0] + "{{x}}" + wrapper[1]))
				want := wrapper[0] + tt.want + wrapper[1]
				if err != nil || string(got) != want {
					t.Fatalf("got=%s err=%v want=%s", got, err, want)
				}
			}
		})
	}
}

func TestJSONStringComposition(t *testing.T) {
	const content = "quote\" slash\\ newline\n雪 {{missing}}"
	encoded, _ := json.Marshal(content)
	c, err := New([]string{"x=" + string(encoded), "n=1.2300e+99", "b=true", "nil=null", "empty="}, nil)
	if err != nil {
		t.Fatal(err)
	}
	source := []byte(`{"name":"pre\n{{x}}\t-{{n}}-{{b}}-{{nil}}-{{empty}}"}`)
	got, err := c.RenderJSON(source)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatal(err)
	}
	want := "pre\n" + content + "\t-1.2300e+99-true-null-"
	if decoded["name"] != want {
		t.Fatalf("got=%q want=%q", decoded["name"], want)
	}
}

func TestJSONPlanBoundariesAndStaticBytes(t *testing.T) {
	for _, source := range []string{`{"a":{}}`, `{"literal":"}}"}`, "{\r\n \"a\": [1,{}],\r\n \"instruction\": \"HTTP 200 GET #\"\r\n}", `"escaped\"\\"`, `null`, `42`, `[]`} {
		c, _ := New(nil, nil)
		plan, err := ParseJSON([]byte(source + "\r\nHTTP 201\r\nGET http://example.test"))
		if err != nil || plan.End != len(source) || len(plan.References) != 0 {
			t.Fatalf("source=%q plan=%+v err=%v", source, plan, err)
		}
		got, err := c.RenderJSON([]byte(source))
		if err != nil || string(got) != source {
			t.Fatalf("static changed: %q err=%v", got, err)
		}
	}
	plan, err := ParseJSON([]byte("{\r\n \"a\":{{x}},\r\n \"b\":\"{{y}}\"\r\n}\nHTTP 200"))
	if err != nil || len(plan.References) != 2 || plan.References[0].Start != 8 || plan.References[0].InString || !plan.References[1].InString {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}

func TestJSONErrorsSafeAndLocated(t *testing.T) {
	for _, source := range []string{`{"{{x}}":1}`, `{ {{x}}:1}`, `{"x":{{bad-name}}}`, `{"x":{{x}`, `{"x":{{x}} "y":1}`, `{"x":}`, `[{{x}},]`, `"bad\q{{x}}"`, `"{{x}}`, `{{}}`, `{"a":"\uZZZZ"}`, "\"\xff\""} {
		plan, err := ParseJSON([]byte(source))
		if err == nil || len(plan.References) != 0 || strings.Contains(err.Error(), "bad-name") {
			t.Fatalf("accepted/unsafe %q: %+v %v", source, plan, err)
		}
	}
	c, _ := New([]string{"object={}", "array=[]"}, nil)
	for _, source := range []string{`["{{object}}"]`, `"{{array}}"`, `{"x":{{missing}}}`, `{} []`} {
		_, err := c.RenderJSON([]byte(source))
		located, ok := err.(*Error)
		if !ok || located.Offset < 0 || strings.Contains(err.Error(), "missing") {
			t.Fatalf("source=%q err=%v", source, err)
		}
	}
}

func FuzzRenderJSON(f *testing.F) {
	for _, seed := range []string{`{{x}}`, `{"a":{{x}}}`, `"prefix{{x}}"`, `{"a":{}}`, `{"a":"\\{{x}}"}`, `{"{{x}}":1}`, "[\r\n{{x}}\r\n]", `"bad\{{x}}"`} {
		f.Add(seed, `"\"\\\n雪{{literal}}"`)
	}
	for _, value := range []string{`null`, `true`, `9007199254740993`, `1.2300e+99`, `[]`, `{"n":[9007199254740993,"{{literal}}"]}`} {
		f.Add(`{"value":{{x}}}`, value)
	}
	f.Fuzz(func(t *testing.T, source, value string) {
		c, err := New([]string{"x=" + value}, []string{"x"})
		if err != nil {
			t.Fatal(err)
		}
		s := c.NewSession()
		captureValue := strings.ToValidUTF8(value, "\ufffd")
		if err := s.Publish([]Capture{{"captured", captureString(captureValue)}}); err != nil {
			t.Fatal(err)
		}
		captured, err := s.RenderJSON([]byte(`{{captured}}`))
		var decoded string
		if err != nil || json.Unmarshal(captured, &decoded) != nil || decoded != captureValue {
			t.Fatal("captured string changed type or content")
		}
		if json.Valid([]byte(value)) {
			if err := s.Publish([]Capture{{"typed", json.RawMessage(value)}}); err != nil {
				t.Fatal(err)
			}
			got, err := s.RenderJSON([]byte(`{{typed}}`))
			if err != nil || !json.Valid(got) {
				t.Fatal("invalid typed capture", err)
			}
		}
		original := []byte(source)
		saved := bytes.Clone(original)
		got, err := c.RenderJSON(original)
		if !bytes.Equal(original, saved) {
			t.Fatal("mutated source")
		}
		if err == nil && !json.Valid(got) {
			t.Fatalf("invalid rendered JSON: %q", got)
		}
		if err != nil && strings.Contains(err.Error(), "literal") {
			t.Fatal("unsafe error")
		}
	})
}
