package runner

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-455,458: the entire compiled Path is operational, on every exit path.
func TestProjectionPlanRemovedFromEveryReport(t *testing.T) {
	for _, tc := range []struct {
		body    string
		outcome Outcome
	}{
		{`{"private_path":[9007199254740992,9007199254740993]}`, Passed},
		{`{"private_path":[9007199254740992,9007199254740992]}`, JSONMismatch},
		{`{"private_path":null}`, CaptureFailed},
		{`{"private_path":[9007199254740992,9007199254740993],"bad":[{},null]}`, CaptureFailed},
	} {
		t.Run(string(tc.outcome), func(t *testing.T) {
			source := "GET http://example.test\nHTTP 200\n[Asserts]\njsonpath \"$.private_path[*]\" == [9007199254740992,9007199254740993]\n[Captures]\nids: jsonpath \"$.private_path[*]\""
			if tc.outcome == CaptureFailed {
				source += "\nbad: jsonpath \"$.bad[*].x\""
			}
			s := parseCaptureScenario(t, source)
			original := s.Entries[0].Assertions[0].JSON.Path
			renderer, _ := template.New([]string{"token=private_path"}, []string{"token", "ids"})
			reports := 0
			check := func(e EntryResult) error {
				reports++
				declaration := e.Entry.Assertions[0].JSON
				if !reflect.DeepEqual(declaration.Path, query.Path{}) || declaration.Expected != nil || len(e.Entry.Captures) != 0 {
					t.Fatalf("operational plan retained: %+v", e.Entry)
				}
				raw, err := json.Marshal(e)
				if err != nil || strings.Contains(string(raw), "private_path") || strings.Contains(string(raw), "900719925474099") {
					t.Fatalf("unsafe report: %s err=%v", raw, err)
				}
				return nil
			}
			r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				return Response{Status: 200, Body: []byte(tc.body)}, nil
			}), renderer, check)
			for range 2 {
				got := r.Run(context.Background(), s)
				if len(got.Entries) != 1 || got.Entries[0].Outcome != tc.outcome {
					t.Fatalf("%+v", got)
				}
				_ = check(got.Entries[0])
				if !reflect.DeepEqual(s.Entries[0].Assertions[0].JSON.Path, original) || !original.Expansive() {
					t.Fatal("source plan mutated")
				}
			}
			if reports != 4 {
				t.Fatalf("reports=%d", reports)
			}
		})
	}
}

// GO-451,455: strict traversal and late conflicts never publish a numeric batch.
func TestProjectionExactBatchAtomicAndProtected(t *testing.T) {
	const numbers = `[9007199254740992,9007199254740993,9223372036854775809,1.0000000000000000000002,1e-99999999999999999999999]`
	for _, late := range []string{"missing: jsonpath \"$.missing[*]\"", "bad: jsonpath \"$.bad[*].x\"", "ids.child: jsonpath \"$.values[0]\"", "previous.child: jsonpath \"$.values[0]\""} {
		c, _ := template.New(nil, []string{"ids"})
		session := c.NewSession()
		if err := session.Publish([]template.Capture{{Name: "previous", Value: json.RawMessage(`7`)}}); err != nil {
			t.Fatal(err)
		}
		s := parseCaptureScenario(t, "GET http://example.test\n[Captures]\nids: jsonpath \"$.values[*]\"\n"+late)
		failure := captureResponse(session, s.Entries[0].Captures, Response{Body: []byte(`{"values":` + numbers + `,"bad":[{"x":1},null]}`)})
		if failure == nil {
			t.Fatal("batch passed")
		}
		if _, err := session.RenderJSON([]byte(`{{ids}}`)); err == nil {
			t.Fatal("partial batch published")
		}
		prior, err := session.RenderJSON([]byte(`{{previous}}`))
		if err != nil || string(prior) != "7" {
			t.Fatalf("prior=%s err=%v", prior, err)
		}
		for _, leaf := range []string{"9007199254740993", "9223372036854775809", "1.0000000000000000000002", "1e-99999999999999999999999"} {
			if session.Sanitize(leaf) != "[oculto]" {
				t.Fatalf("discovered number unprotected: %s", leaf)
			}
		}
	}
}
