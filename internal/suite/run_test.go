package suite

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
)

type executeFunc func(context.Context, runner.Request) (runner.Response, error)

func (f executeFunc) Execute(ctx context.Context, r runner.Request) (runner.Response, error) {
	return f(ctx, r)
}

func TestSuiteContinuationCountsAndClock(t *testing.T) {
	for _, mode := range []string{"pass", "assertion", "transport", "parse", "capture", "unvalidated", "pre-send"} {
		for _, fast := range []bool{false, true} {
			t.Run(mode+"/fast="+map[bool]string{false: "false", true: "true"}[fast], func(t *testing.T) {
				dir := t.TempDir()
				fixture(t, dir, "01.hurl", "GET http://example.test/first\nHTTP 200")
				text := "GET http://example.test/second\nHTTP 200\nGET http://example.test/blocked"
				switch mode {
				case "parse":
					text += "\n[Unsupported]"
				case "capture":
					text = "GET http://example.test/second\nHTTP 200\n[Captures]\nid: jsonpath \"$.missing\"\nGET http://example.test/blocked"
				case "unvalidated":
					text = "GET http://example.test/second"
				case "pre-send":
					text = "GET http://example.test/{{missing}}\nGET http://example.test/blocked"
				}
				fixture(t, dir, "02.hurl", text)
				fixture(t, dir, "03.hurl", "GET http://example.test/last\nHTTP 200")
				var calls []string
				s := New(runner.New(executeFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
					calls = append(calls, req.URL)
					if strings.HasSuffix(req.URL, "/second") {
						if mode == "transport" {
							return runner.Response{}, errors.New("safe private cause")
						}
						if mode == "assertion" {
							return runner.Response{Status: 503}, nil
						}
					}
					return runner.Response{Status: 200, Body: []byte(`{}`)}, nil
				}), nil))
				tick := time.Unix(0, 0)
				s.now = func() time.Time { tick = tick.Add(time.Second); return tick }
				files, err := s.Discover(context.Background(), []string{dir})
				if err != nil {
					t.Fatal(err)
				}
				var reports []State
				result := s.Run(context.Background(), files, Options{FailFast: fast}, func(file FileResult, _ runner.Result, _ error) error {
					reports = append(reports, file.State)
					return nil
				})
				fail := mode != "pass" && mode != "unvalidated"
				wantCode := 0
				if fail {
					wantCode = 2
					if mode == "assertion" {
						wantCode = 1
					}
				}
				wantLast := Passed
				if fast && fail {
					wantLast = Skipped
				}
				if result.Code != wantCode || len(result.Files) != 3 || result.Files[2].State != wantLast || !reflect.DeepEqual(reports, []State{Passed, result.Files[1].State, wantLast}) {
					t.Fatalf("result=%+v reports=%v calls=%v", result, reports, calls)
				}
				second := result.Files[1]
				if second.Known == (mode == "parse") {
					t.Fatalf("known=%v mode=%s", second.Known, mode)
				}
				if mode == "capture" && (second.Requests.Validated != 1 || second.Successful != 0 || second.RequestErrors != 1) {
					t.Fatalf("status is not entire success: %+v", second)
				}
				if mode == "unvalidated" && (second.Requests.WithoutValidation != 1 || second.Requests.Validated != 0 || second.Successful != 1) {
					t.Fatalf("without assertion: %+v", second)
				}
				if mode == "parse" || mode == "pre-send" {
					if second.Requests.Started != 0 {
						t.Fatalf("phantom send: %+v", second)
					}
				}
				if fast && fail && (result.Files[2].Known || result.Summary.Skipped != 1) {
					t.Fatalf("unparsed skipped: %+v", result)
				}
				if second.Duration != time.Second || result.Summary.Duration <= 0 {
					t.Fatalf("clock ignored: %+v", result)
				}
				if fail {
					for _, call := range calls {
						if strings.HasSuffix(call, "/blocked") {
							t.Fatalf("continued failed file: %v", calls)
						}
					}
				}
			})
		}
	}
}

func TestSuiteErrorPrecedenceAndPerFileValidation(t *testing.T) {
	dir := t.TempDir()
	fixture(t, dir, "01.hurl", "GET http://example.test/first\nHTTP 200")
	fixture(t, dir, "02.hurl", "GET http://example.test/not-sent\nHEAD http://example.test/invalid")
	fixture(t, dir, "03.hurl", "GET http://example.test/last")
	var calls []string
	s := New(runner.New(executeFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
		calls = append(calls, req.URL)
		return runner.Response{Status: 503}, nil
	}), nil))
	files, err := s.Discover(context.Background(), []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	result := s.Run(context.Background(), files, Options{}, nil)
	if result.Code != 2 || result.Summary.Failed != 1 || result.Summary.Errors != 1 || result.Summary.Passed != 1 || result.Summary.Requests.Started != 2 || !reflect.DeepEqual(calls, []string{"http://example.test/first", "http://example.test/last"}) {
		t.Fatalf("result=%+v calls=%v", result, calls)
	}
}

func TestSuiteCancellationAndPresentationAreGlobal(t *testing.T) {
	for _, mode := range []string{"during request", "between files", "entry writer", "file writer", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			fixture(t, dir, "01.hurl", "GET http://example.test/first\nHTTP 200")
			fixture(t, dir, "02.hurl", "GET http://example.test/last\nHTTP 200")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls, reports := 0, 0
			cause := errors.New("private writer failure")
			s := New(runner.New(executeFunc(func(_ context.Context, _ runner.Request) (runner.Response, error) {
				calls++
				if mode == "during request" {
					cancel()
					return runner.Response{}, ctx.Err()
				}
				if mode == "timeout" && calls == 1 {
					return runner.Response{}, context.DeadlineExceeded
				}
				return runner.Response{Status: 200}, nil
			}), func(runner.EntryResult) error {
				if mode == "entry writer" {
					return cause
				}
				return nil
			}))
			files, err := s.Discover(ctx, []string{dir})
			if err != nil {
				t.Fatal(err)
			}
			result := s.Run(ctx, files, Options{}, func(FileResult, runner.Result, error) error {
				reports++
				if mode == "between files" {
					cancel()
				}
				if mode == "file writer" {
					return cause
				}
				return nil
			})
			wantCalls := 1
			if mode == "timeout" {
				wantCalls = 2
			}
			if result.Code != 2 || calls != wantCalls || len(result.Files) != 2 {
				t.Fatalf("result=%+v calls=%d reports=%d", result, calls, reports)
			}
			if mode == "entry writer" || mode == "file writer" {
				wantReports := 0
				if mode == "file writer" {
					wantReports = 1
				}
				if reports != wantReports || !errors.Is(result.ReportErr, cause) {
					t.Fatalf("result=%+v reports=%d", result, reports)
				}
			}
			if mode != "timeout" && result.Files[1].State != Skipped {
				t.Fatalf("next started: %+v", result)
			}
		})
	}
}

func TestDiscoveryCancellationAndLateLoadFailure(t *testing.T) {
	dir := t.TempDir()
	fixture(t, dir, "01.hurl", "GET http://example.test/first")
	second := fixture(t, dir, "02.hurl", "GET http://example.test/second")
	fixture(t, dir, "03.hurl", "GET http://example.test/last")
	ctx, cancel := context.WithCancel(context.Background())
	s := New(nil)
	original := s.readDir
	s.readDir = func(path string) ([]os.DirEntry, error) { cancel(); return original(path) }
	if files, err := s.Discover(ctx, []string{dir}); files != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("files=%v err=%v", files, err)
	}
	for _, mode := range []string{"open", "read", "close"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			s := New(runner.New(executeFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200}, nil
			}), nil))
			files, err := s.Discover(context.Background(), []string{dir})
			if err != nil {
				t.Fatal(err)
			}
			original := s.open
			info, err := os.Stat(second)
			if err != nil {
				t.Fatal(err)
			}
			cause := errors.New("late private cause")
			s.open = func(path string) (scenarioFile, error) {
				if path != second {
					return original(path)
				}
				if mode == "open" {
					return nil, cause
				}
				f := &faultFile{Reader: strings.NewReader("GET http://example.test/second"), info: info}
				if mode == "read" {
					f.Reader = io.MultiReader(f.Reader, readFailure{cause})
				} else {
					f.closeErr = cause
				}
				return f, nil
			}
			result := s.Run(context.Background(), files, Options{}, func(file FileResult, _ runner.Result, err error) error {
				if file.State == Errored && !errors.Is(err, cause) {
					t.Fatalf("cause lost: %v", err)
				}
				return nil
			})
			if result.Code != 2 || calls != 2 || result.Files[1].Requests.Started != 0 || result.Files[2].State != Passed {
				t.Fatalf("result=%+v calls=%d", result, calls)
			}
		})
	}
}

func TestSuiteProtectedErrorCauseAndNonnegativeDuration(t *testing.T) {
	dir := t.TempDir()
	fixture(t, dir, "fiction-secret.hurl", "GET http://example.test")
	cause := errors.New("private raw transport cause")
	s := NewWithProtection(runner.New(executeFunc(func(context.Context, runner.Request) (runner.Response, error) { return runner.Response{}, cause }), nil), func(text string) string { return strings.ReplaceAll(text, "fiction-secret", "[oculto]") })
	now := time.Unix(20, 0)
	s.now = func() time.Time { now = now.Add(-time.Second); return now }
	files, err := s.Discover(context.Background(), []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	result := s.Run(context.Background(), files, Options{}, func(file FileResult, run runner.Result, _ error) error {
		if strings.Contains(file.Name, "fiction-secret") || !errors.Is(run.Err, cause) || strings.Contains(run.Err.Error(), cause.Error()) {
			t.Fatalf("file=%+v err=%v", file, run.Err)
		}
		return nil
	})
	if result.Code != 2 || result.Summary.Duration != 0 || result.Files[0].Duration != 0 {
		t.Fatalf("result=%+v", result)
	}
}

func TestSuiteStatusValidationDoesNotImplyRequestSuccess(t *testing.T) {
	// Header assertion fails after numeric status passed; the aggregate must not
	// infer request success from Summary.Validated.
	dir := t.TempDir()
	fixture(t, dir, "input.hurl", "GET http://example.test\nHTTP 200\n[Asserts]\nheader \"X-Test\" == \"expected\"")
	renderer, err := template.New(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := New(runner.NewWithRenderer(executeFunc(func(context.Context, runner.Request) (runner.Response, error) {
		return runner.Response{Status: 200, Headers: []query.HeaderValue{{Name: "X-Test", Value: "other"}}}, nil
	}), renderer, nil))
	files, err := s.Discover(context.Background(), []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	result := s.Run(context.Background(), files, Options{}, nil)
	if result.Code != 1 || result.Summary.Requests.Validated != 1 || result.Summary.Successful != 0 || result.Summary.AssertionFailures != 1 {
		t.Fatalf("result=%+v", result)
	}
}
