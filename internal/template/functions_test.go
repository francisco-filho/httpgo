package template

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

func fixedClock(t *testing.T) func() time.Time {
	t.Helper()
	zone := time.FixedZone("-03", -3*60*60)
	return func() time.Time {
		return time.Date(2025, 11, 16, 9, 34, 56, 987654321, zone)
	}
}

func clocked(t *testing.T, assignments, secrets []string) *Context {
	t.Helper()
	c, err := NewWithClock(assignments, secrets, fixedClock(t))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestFunctionsResolveTextContexts(t *testing.T) {
	for _, tc := range []struct {
		name, source, want string
	}{
		{"date", "{{newDate}}", "2025-11-16T09:34:56-03:00"},
		{"date spaces", "{{ newDate }}", "2025-11-16T09:34:56-03:00"},
		{"date tabs", "{{\tnewDate\t}}", "2025-11-16T09:34:56-03:00"},
		{"date composed", "at={{newDate}}&note=ok", "at=2025-11-16T09:34:56-03:00&note=ok"},
		{"date with variable", "{{prefix}}-{{newDate}}", "run-2025-11-16T09:34:56-03:00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := clocked(t, []string{"prefix=run"}, nil)
			got, err := c.Render(tc.source)
			if err != nil || got.Value != tc.want || got.Display != tc.want {
				t.Fatalf("source=%q got=%+v err=%v want=%q", tc.source, got, err, tc.want)
			}
		})
	}
}

func TestFunctionsResolveEveryOccurrence(t *testing.T) {
	c := clocked(t, nil, nil)
	first, err := c.Render("{{newUuid}}")
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Render("{{newUuid}}")
	if err != nil {
		t.Fatal(err)
	}
	combined, err := c.Render("{{newUuid}}/{{newUuid}}")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(combined.Value, "/")
	if len(parts) != 2 || first.Value == second.Value || parts[0] == parts[1] {
		t.Fatalf("first=%q second=%q combined=%q", first.Value, second.Value, combined.Value)
	}
	for _, value := range append(parts, first.Value, second.Value) {
		if !uuidPattern.MatchString(value) {
			t.Fatalf("non v4 canonical uuid: %q", value)
		}
	}
}

func TestFunctionsKeepNonRecursiveContent(t *testing.T) {
	c := clocked(t, []string{`literal={{newUuid}}`}, nil)
	got, err := c.Render("{{literal}}")
	if err != nil || got.Value != "{{newUuid}}" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestFunctionsResolveJSONContexts(t *testing.T) {
	c := clocked(t, []string{"label=pedido"}, nil)
	for _, tc := range []struct {
		name, source, want string
		decode             func(*testing.T, []byte) string
	}{
		{
			name:   "complete value becomes JSON string",
			source: `{"requestId":{{newUuid}}}`,
			decode: func(t *testing.T, body []byte) string {
				t.Helper()
				var decoded map[string]json.RawMessage
				if err := json.Unmarshal(body, &decoded); err != nil {
					t.Fatal(err)
				}
				var value string
				if err := json.Unmarshal(decoded["requestId"], &value); err != nil {
					t.Fatalf("complete value is not a JSON string: %s", body)
				}
				return value
			},
		},
		{
			name:   "inside string",
			source: `{"label":"pedido-{{newUuid}}"}`,
			decode: func(t *testing.T, body []byte) string {
				t.Helper()
				var decoded map[string]string
				if err := json.Unmarshal(body, &decoded); err != nil {
					t.Fatal(err)
				}
				return strings.TrimPrefix(decoded["label"], "pedido-")
			},
		},
		{
			name:   "composed with date and variable",
			source: `{"createdAt":"{{newDate}}","label":"{{label}}-{{newUuid}}"}`,
			decode: func(t *testing.T, body []byte) string {
				t.Helper()
				var decoded map[string]string
				if err := json.Unmarshal(body, &decoded); err != nil {
					t.Fatal(err)
				}
				if decoded["createdAt"] != "2025-11-16T09:34:56-03:00" {
					t.Fatalf("createdAt=%q body=%s", decoded["createdAt"], body)
				}
				return strings.TrimPrefix(decoded["label"], "pedido-")
			},
		},
		{
			name:   "array element",
			source: `[{{newUuid}},{{newUuid}}]`,
			decode: func(t *testing.T, body []byte) string {
				t.Helper()
				var decoded []string
				if err := json.Unmarshal(body, &decoded); err != nil {
					t.Fatalf("array element is not a string: %s", body)
				}
				if len(decoded) != 2 || decoded[0] == decoded[1] {
					t.Fatalf("values=%v", decoded)
				}
				return decoded[0]
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := c.RenderJSON([]byte(tc.source))
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(body) {
				t.Fatalf("invalid JSON: %s", body)
			}
			if value := tc.decode(t, body); !uuidPattern.MatchString(value) {
				t.Fatalf("non v4 canonical uuid: %q body=%s", value, body)
			}
		})
	}
}

func TestFunctionDateKeepsLocalOffsetAndSeconds(t *testing.T) {
	for _, tc := range []struct {
		name string
		now  time.Time
		want string
	}{
		{"negative offset", time.Date(2025, 11, 16, 9, 34, 56, 750, time.FixedZone("-03", -3*60*60)), "2025-11-16T09:34:56-03:00"},
		{"positive offset", time.Date(2025, 1, 2, 3, 4, 5, 0, time.FixedZone("+0530", 5*3600+1800)), "2025-01-02T03:04:05+05:30"},
		{"UTC", time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC), "2025-06-01T00:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewWithClock(nil, nil, func() time.Time { return tc.now })
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.Render("{{newDate}}")
			if err != nil || got.Value != tc.want {
				t.Fatalf("got=%+v err=%v want=%q", got, err, tc.want)
			}
			if _, err := time.Parse(time.RFC3339, got.Value); err != nil {
				t.Fatalf("not RFC 3339: %v", err)
			}
		})
	}
}

func TestFunctionClockReachesSessions(t *testing.T) {
	c := clocked(t, nil, nil)
	session := c.NewSession()
	got, err := session.Render("{{newDate}}")
	if err != nil || got.Value != "2025-11-16T09:34:56-03:00" {
		t.Fatalf("session render=%+v err=%v", got, err)
	}
	view, err := session.Stage([]Capture{{Name: "value", Value: json.RawMessage(`"x"`)}})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := view.Render("{{newDate}}-{{value}}"); err != nil || got.Value != "2025-11-16T09:34:56-03:00-x" {
		t.Fatalf("staged render=%+v err=%v", got, err)
	}
}

func TestNilClockFallsBackToCurrentTime(t *testing.T) {
	c := clocked(t, nil, nil)
	if _, err := c.Render("{{newDate}}"); err != nil {
		t.Fatal(err)
	}
	fallback, err := NewWithClock(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fallback.Render("{{newDate}}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := time.Parse(time.RFC3339, got.Value); err != nil {
		t.Fatalf("not RFC 3339: %v", err)
	}
}

func TestReservedFunctionNamesAreRejected(t *testing.T) {
	for _, tc := range []struct {
		name              string
		assignments       []string
		secrets           []string
		wantDetail        string
		rejectsAssignment bool
	}{
		{name: "variable newDate", assignments: []string{"newDate=2025-01-01T00:00:00Z"}, wantDetail: "newDate", rejectsAssignment: true},
		{name: "variable newUuid", assignments: []string{"newUuid=00000000-0000-4000-8000-000000000000"}, wantDetail: "newUuid", rejectsAssignment: true},
		{name: "secret newUuid", secrets: []string{"newUuid"}, wantDetail: "newUuid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.assignments, tc.secrets)
			if err == nil {
				t.Fatalf("accepted %v %v", tc.assignments, tc.secrets)
			}
			if !strings.Contains(err.Error(), ReservedDetail) || !strings.Contains(err.Error(), tc.wantDetail) {
				t.Fatalf("unsafe or unclear error=%v", err)
			}
		})
	}
	for _, name := range []string{"newDate.x", "xnewDate", "newdate", "NEWUID", "newUuid.x.y"} {
		if ReservedFunction(name) {
			t.Fatalf("reserved %q", name)
		}
		if _, err := New([]string{name + "=value"}, nil); err != nil {
			t.Fatalf("rejected descendant or similar name %q: %v", name, err)
		}
	}
}

func TestReservedFunctionNamesAreRejectedInCaptures(t *testing.T) {
	for _, name := range []string{"newDate", "newUuid"} {
		t.Run(name, func(t *testing.T) {
			c, err := New(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			batch := []Capture{{Name: "safe", Value: json.RawMessage(`"ok"`)}, {Name: name, Value: json.RawMessage(`"x"`)}}
			_, stageErr := c.Stage(batch)
			var reserved *ReservedError
			if !errors.As(stageErr, &reserved) || reserved.Index != 1 {
				t.Fatalf("stage err=%v", stageErr)
			}
			publishErr := c.Publish(batch)
			if !errors.As(publishErr, &reserved) || reserved.Index != 1 {
				t.Fatalf("publish err=%v", publishErr)
			}
			if strings.Contains(publishErr.Error(), `"x"`) {
				t.Fatalf("unsafe error=%v", publishErr)
			}
			if got, err := c.Render("{{safe}}"); err == nil {
				t.Fatalf("published after rejection: %+v", got)
			}
		})
	}
}

func TestUnknownFunctionLikeNamesRemainVariables(t *testing.T) {
	c := clocked(t, nil, nil)
	_, err := c.Render("{{newFoo}}")
	var located *Error
	if !errors.As(err, &located) || !strings.Contains(located.Detail, "ausente") {
		t.Fatalf("err=%v", err)
	}
	if _, err := c.RenderJSON([]byte(`{"a":{{newFoo}}}`)); !errors.As(err, &located) || !strings.Contains(located.Detail, "ausente") {
		t.Fatalf("json err=%v", err)
	}
	for _, source := range []string{"{{newDate", "{{newUuid", "{{new-uuid}}", "{{newUuid()}}"} {
		if _, err := c.Render(source); !errors.As(err, &located) {
			t.Fatalf("source=%q err=%v", source, err)
		}
	}
}
