package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-409,414: no operational plans, operands or protected summaries survive the
// presentation boundary. Rebinding the reusable scenario is session-local.
func TestDynamicFilterSafeResultsAndReuse(t *testing.T) {
	for _, mode := range []string{"success", "mismatch", "type", "missing", "capture", "pending"} {
		t.Run(mode, func(t *testing.T) {
			source := "GET http://example.test\nHTTP 200\n[Asserts]\n" + `jsonpath "$[?(@.key=='prefix-{{key}}')].v" == [1,2]` + "\n[Captures]\nselected: jsonpath \"$[?(@.key=='prefix-{{key}}')].v\""
			if mode == "capture" {
				source += "\nmissing: jsonpath \"$[?(@.key=={{absent}})]\""
			}
			s := dynamicScenario(t, source)
			before := dynamicScenario(t, source)
			for _, private := range []string{"first-private", "second-private"} {
				value := `"` + private + `"`
				if mode == "type" {
					value = `{"nested":"` + private + `"}`
				}
				secrets := []string{"key"}
				if mode == "pending" {
					secrets = []string{"selected"}
				}
				c, _ := template.New([]string{"key=" + value}, secrets)
				if mode == "missing" {
					c, _ = template.New(nil, secrets)
				}
				body := `[{"key":"prefix-` + private + `","v":[1,2]}]`
				if mode == "mismatch" || mode == "pending" {
					body = `[{"key":"prefix-` + private + `","v":[3,4]}]`
				}
				checks := 0
				check := func(e EntryResult) error {
					checks++
					a := e.Entry.Assertions[0].JSON
					if !reflect.DeepEqual(a.Path, query.Path{}) || a.ExpectedTemplate != nil || a.Expected != nil || len(e.Entry.Captures) != 0 {
						t.Fatalf("retained metadata: %+v", a)
					}
					raw, _ := json.Marshal(e)
					text := fmt.Sprintf("%+v %+v %+v", a, e.JSONError, e.JSONFailure) + string(raw)
					for _, bad := range []string{private, "{{key}}", "{{absent}}", "[1,2]", "[3,4]"} {
						if strings.Contains(text, bad) {
							t.Fatalf("unsafe result: %s", text)
						}
					}
					if e.JSONFailure != nil && (e.JSONFailure.Actual != "[oculto]" || e.JSONFailure.Expected != "[oculto]") {
						t.Fatal("unprotected provenance")
					}
					return nil
				}
				result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
					return Response{Status: 200, Body: []byte(body)}, nil
				}), c, check).Run(context.Background(), s)
				want := Passed
				switch mode {
				case "mismatch", "pending":
					want = JSONMismatch
				case "type", "missing":
					want = JSONFailed
				case "capture":
					want = CaptureFailed
				}
				if result.Entries[0].Outcome != want {
					t.Fatalf("result=%+v err=%v", result, result.Err)
				}
				_ = check(result.Entries[0])
				if checks != 2 || !reflect.DeepEqual(s, before) {
					t.Fatalf("checks=%d mutated=%v", checks, !reflect.DeepEqual(s, before))
				}
			}
		})
	}
}

// GO-409: selecting through a secret operand marks the derived capture before
// publication, so later assertions also conceal non-overlapping actual values.
func TestDynamicFilterDerivedSecret(t *testing.T) {
	c, _ := template.New([]string{`key="private-key"`}, []string{"key"})
	s := dynamicScenario(t, "GET http://example.test/select\nHTTP 200\n[Captures]\n"+`selected: jsonpath "$[?(@.key=={{key}})].id"`+"\nGET http://example.test/check\nHTTP 200\n[Asserts]\nheader \"X\" == \"prefix-{{selected}}\"")
	result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
		return Response{Status: 200, Body: []byte(`[{"key":"private-key","id":"derived-private"}]`), Headers: []query.HeaderValue{{Name: "X", Value: "unrelated-private"}}}, nil
	}), c, nil).Run(context.Background(), s)
	if !result.Diverged || result.Entries[1].HeaderFailure.Expected != "[oculto]" || result.Entries[1].HeaderFailure.Actual != "[oculto]" {
		t.Fatalf("result=%+v", result)
	}
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "derived-private") || strings.Contains(string(raw), "unrelated-private") {
		t.Fatalf("unsafe: %s", raw)
	}
}

// GO-412–413: transport errors never evaluate a partial response; report errors
// retain category 2 and stop without retries, capture or subsequent sends.
func TestDynamicFilterExternalFailureOrder(t *testing.T) {
	for _, mode := range []string{"transport", "cancel", "operand", "report"} {
		t.Run(mode, func(t *testing.T) {
			calls, reports := 0, 0
			cause := errors.New("private I/O cause")
			s := dynamicScenario(t, "GET http://example.test\n[Options]\nretry: 2\nretry-interval: 0\nHTTP 200\n[Asserts]\n"+`jsonpath "$[?(@.x=={{missing}})]" not exists`+"\nGET http://example.test/never")
			result := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				if mode == "transport" {
					return Response{Status: 200, Body: []byte(`[]`)}, cause
				}
				if mode == "cancel" {
					return Response{}, context.Canceled
				}
				return Response{Status: 200, Body: []byte(`[]`)}, nil
			}), func(EntryResult) error {
				reports++
				if mode == "report" {
					return cause
				}
				return nil
			}).Run(context.Background(), s)
			if calls != 1 || reports != 1 || result.Diverged || result.Err == nil || result.Summary.NotStarted != 1 {
				t.Fatalf("calls=%d reports=%d result=%+v", calls, reports, result)
			}
			if mode == "report" && !errors.Is(result.ReportErr, cause) {
				t.Fatal(result.ReportErr)
			}
			if mode == "transport" && (!errors.Is(result.Err, cause) || result.Summary.Completed != 0 || result.Entries[0].JSONError != nil) {
				t.Fatal("evaluated partial response")
			}
		})
	}
}
