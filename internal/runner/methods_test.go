package runner

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
)

func TestMixedMethodsForwardedAndInterrupted(t *testing.T) {
	scenario, err := parser.Parse("mixed.hurl", strings.NewReader("POST http://example.test/products\nContent-Type: application/json\n{\"name\":\"Pizza\",\"price\":4500}\nHTTP 201\nPUT http://example.test/products/1\n[1, 2]\nHTTP 200\nPATCH http://example.test/products/1\ntrue\nDELETE http://example.test/products/1\nHTTP 204\nGET http://example.test/missing\nHTTP 404"))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"success", "mismatch", "transport", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			failure := errors.New("test failure")
			statuses := []int{201, 200, 500, 204, 404}
			executor := executorFunc(func(gotCtx context.Context, req Request) (Response, error) {
				entry := scenario.Entries[calls]
				if gotCtx != ctx || !reflect.DeepEqual(req, Request{Method: entry.Method, URL: entry.URL, Headers: entry.Headers, Body: entry.Body}) {
					t.Fatalf("request not forwarded unchanged: %+v", req)
				}
				calls++
				if calls == 2 {
					switch mode {
					case "mismatch":
						return Response{Status: 503}, nil
					case "transport":
						return Response{}, failure
					case "cancel":
						cancel()
						return Response{}, gotCtx.Err()
					}
				}
				return Response{Status: statuses[calls-1]}, nil
			})
			got := New(executor, nil).Run(ctx, scenario)
			if mode == "success" {
				if calls != 5 || got.Err != nil || got.Diverged || got.Summary.WithoutValidation != 1 || got.Entries[2].Outcome != Unvalidated {
					t.Fatalf("calls=%d,result=%+v", calls, got)
				}
			} else {
				if calls != 2 || got.Summary.NotStarted != 3 {
					t.Fatalf("calls=%d,result=%+v", calls, got)
				}
				if mode == "mismatch" && !got.Diverged || mode == "transport" && !errors.Is(got.Err, failure) || mode == "cancel" && !errors.Is(got.Err, context.Canceled) {
					t.Fatalf("result=%+v", got)
				}
			}
		})
	}
}
