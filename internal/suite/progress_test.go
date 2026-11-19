package suite

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestFileLifecycleNotifications(t *testing.T) {
	for _, mode := range []string{"pass", "retry", "pre-send", "parse", "loading failure", "loaded failure", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			text := "GET http://example.test/one\nHTTP 200\nGET http://example.test/two"
			switch mode {
			case "retry":
				text = "GET http://example.test/one\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200"
			case "pre-send":
				text = "GET http://example.test/{{missing}}"
			case "parse":
				text += "\n[Unsupported]"
			}
			fixture(t, dir, "01.hurl", text)
			fixture(t, dir, "02.hurl", "GET http://example.test/not-reached")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var events []string
			calls, terminals, loads := 0, 0, 0
			cause := errors.New("presentation failure")
			s := New(runner.New(executeFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				events = append(events, "send")
				if mode == "cancel" {
					cancel()
					return runner.Response{}, ctx.Err()
				}
				if mode == "retry" && calls == 1 {
					return runner.Response{Status: 503}, nil
				}
				return runner.Response{Status: 200}, nil
			}), func(runner.EntryResult) error { terminals++; events = append(events, "step"); return nil }))
			files, err := s.Discover(ctx, []string{dir})
			if err != nil {
				t.Fatal(err)
			}
			original := s.open
			s.open = func(path string) (scenarioFile, error) { loads++; return original(path) }
			result := s.Run(ctx, files, Options{FailFast: true,
				Loading: func(string) error {
					events = append(events, "loading")
					if mode == "loading failure" {
						return cause
					}
					return nil
				},
				Loaded: func(_ string, total int) error {
					events = append(events, "loaded")
					if total < 1 {
						t.Fatal("invalid total")
					}
					if mode == "loaded failure" {
						return cause
					}
					return nil
				},
			}, func(file FileResult, _ runner.Result, _ error) error {
				events = append(events, string(file.State))
				return nil
			})
			if mode == "loading failure" || mode == "loaded failure" {
				want := []string{"loading"}
				if mode == "loaded failure" {
					want = append(want, "loaded")
				}
				if !reflect.DeepEqual(events, want) || !errors.Is(result.ReportErr, cause) || calls != 0 || result.Code != 2 {
					t.Fatalf("events=%v result=%+v calls=%d", events, result, calls)
				}
				return
			}
			if mode == "parse" {
				if loads != 1 || calls != 0 || !reflect.DeepEqual(events, []string{"loading", string(Errored), string(Skipped)}) {
					t.Fatalf("events=%v loads=%d calls=%d", events, loads, calls)
				}
				return
			}
			if len(events) < 2 || events[0] != "loading" || events[1] != "loaded" {
				t.Fatalf("events=%v", events)
			}
			if mode == "retry" && (calls != 3 || terminals != 2) {
				t.Fatalf("calls=%d terminals=%d events=%v", calls, terminals, events)
			}
			if mode == "pre-send" && (calls != 0 || terminals != 1 || loads != 1 || result.Summary.Processed != 1) {
				t.Fatalf("result=%+v calls=%d terminals=%d loads=%d", result, calls, terminals, loads)
			}
			if mode == "cancel" && (calls != 1 || terminals != 1 || loads != 1 || result.Summary.Skipped != 1) {
				t.Fatalf("result=%+v events=%v", result, events)
			}
		})
	}
}

func TestCancelledBeforeLoadingDoesNotNotify(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := New(nil)
	result := s.Run(ctx, []File{{Name: "not-reached.hurl"}}, Options{Loading: func(string) error { t.Fatal("loading skipped file"); return nil }, Loaded: func(string, int) error { t.Fatal("parsing skipped file"); return nil }}, nil)
	if result.Code != 2 || result.Summary.Processed != 0 || result.Summary.Skipped != 1 {
		t.Fatalf("result=%+v", result)
	}
}
