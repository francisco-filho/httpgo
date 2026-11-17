package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

// GO-437: exact and both hierarchical directions, without an earlier collision
// accidentally masking the intended one. Other captures remain unpublished.
func TestCountCaptureConflictDirections(t *testing.T) {
	for _, source := range []string{"initial", "previous", "batch"} {
		for _, names := range [][2]string{{"saved", "saved"}, {"saved", "saved.child"}, {"saved.child", "saved"}} {
			t.Run(fmt.Sprintf("%s/%s/%s", source, names[0], names[1]), func(t *testing.T) {
				var assignments []string
				if source == "initial" {
					assignments = []string{names[0] + "=7"}
				}
				c, _ := template.New(assignments, nil)
				session := c.NewSession()
				if source == "previous" {
					if err := session.Publish([]template.Capture{{Name: names[0], Value: json.RawMessage(`7`)}}); err != nil {
						t.Fatal(err)
					}
				}
				declarations := "fresh: jsonpath \"$\" count\n"
				if source == "batch" {
					declarations += names[0] + ": jsonpath \"$\" count\n"
				}
				declarations += names[1] + ": jsonpath \"$\" count\n"
				s := parseCaptureScenario(t, "GET http://example.test/\nHTTP 200\n[Captures]\n"+declarations+"GET http://example.test/never")
				calls := 0
				got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
					calls++
					return Response{Status: 200, Body: []byte(`[null,null]`)}, nil
				}), observedHeaderSession{session}, nil).Run(context.Background(), s)
				if calls != 1 || got.Err == nil || got.Entries[0].Outcome != CaptureFailed {
					t.Fatalf("calls=%d result=%+v", calls, got)
				}
				if _, err := session.Render("{{fresh}}"); err == nil {
					t.Fatal("partial publication")
				}
				if source != "batch" {
					if value, err := session.Render("{{" + names[0] + "}}"); err != nil || value.Value != "7" {
						t.Fatalf("previous overwritten: %+v %v", value, err)
					}
				}
			})
		}
	}
}
