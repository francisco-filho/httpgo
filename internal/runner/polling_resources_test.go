package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPollingCancellationAndTimer(t *testing.T) {
	for _, mode := range []string{"before", "wait", "timer-race"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls, reports, waits := 0, 0, 0
			r := New(executorFunc(func(context.Context, Request) (Response, error) { calls++; return Response{Status: 503}, nil }), func(entry EntryResult) error {
				reports++
				if entry.Outcome != Cancelled || entry.Status != 0 || entry.Attempts != 1 {
					t.Fatalf("entry=%+v", entry)
				}
				return nil
			})
			r.wait = func(ctx context.Context, d time.Duration) error {
				waits++
				cancel()
				if mode == "timer-race" {
					return nil
				}
				return waitInterval(ctx, d)
			}
			if mode == "before" {
				cancel()
			}
			got := r.Run(ctx, pollingScenario(t, "GET http://example.test/job\n[Options]\nretry: 3\nHTTP 200\nGET http://example.test/never\n"))
			want := 1
			if mode == "before" {
				want = 0
			}
			if !errors.Is(got.Err, context.Canceled) || got.Diverged || calls != want || reports != want || waits != want || got.Attempts.Started != want || got.Summary.Started != want || got.Summary.Completed != want {
				t.Fatalf("calls=%d reports=%d waits=%d result=%+v", calls, reports, waits, got)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- waitInterval(ctx, time.Hour) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("wait=%v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timer did not cancel")
	}
	if err := waitInterval(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if err := waitInterval(context.Background(), time.Nanosecond); err != nil {
		t.Fatal(err)
	}
}

func TestPollingReportFailureDoesNotStartNextEntry(t *testing.T) {
	for _, success := range []bool{false, true} {
		calls, reports := 0, 0
		cause := errors.New("report failure")
		r := New(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			status := 503
			if success && calls == 3 {
				status = 200
			}
			return Response{Status: status}, nil
		}), func(entry EntryResult) error {
			reports++
			if entry.Attempts != 3 {
				t.Fatalf("entry=%+v", entry)
			}
			return cause
		})
		got := r.Run(context.Background(), pollingScenario(t, "GET http://example.test/job\n[Options]\nretry: 2\nretry-interval: 0\nHTTP 200\nGET http://example.test/never\n"))
		if !errors.Is(got.ReportErr, cause) || calls != 3 || reports != 1 || got.Attempts.Started != 3 || got.Summary.NotStarted != 1 {
			t.Fatalf("result=%+v calls=%d reports=%d", got, calls, reports)
		}
	}
}

func TestPollingReopensMultipartBeforeWaiting(t *testing.T) {
	for _, mode := range []string{"change", "close-error", "read-error"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "upload")
			if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
				t.Fatal(err)
			}
			scenario := pollingScenario(t, "POST http://example.test/job\n[Options]\nretry: 2\nretry-interval: 0\n[MultipartFormData]\nlabel: same\nfile: file,upload;\nHTTP 200\n")
			scenario.OriginDir = dir
			calls, opens, waits := 0, 0, 0
			var source *faultyUpload
			cause := errors.New("resource failure")
			r := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				if source.closed {
					t.Fatal("source prematurely closed")
				}
				data, err := io.ReadAll(req.Multipart.Source)
				want := "first"
				if calls == 2 {
					want = "second"
				}
				if err != nil || string(data) != want {
					t.Fatalf("data=%q err=%v", data, err)
				}
				if mode == "read-error" {
					return Response{}, cause
				}
				status := 503
				if calls == 2 {
					status = 200
				}
				return Response{Status: status}, nil
			}), nil)
			r.openUpload = func(path string) (uploadFile, error) {
				if source != nil && !source.closed {
					t.Fatal("previous upload still open")
				}
				file, err := os.Open(path)
				if err != nil {
					return nil, err
				}
				opens++
				source = &faultyUpload{observedUpload: &observedUpload{File: file}}
				if mode == "close-error" {
					source.closeErr = cause
				}
				return source, nil
			}
			r.wait = func(context.Context, time.Duration) error {
				waits++
				if !source.closed {
					t.Fatal("upload open during wait")
				}
				return os.WriteFile(path, []byte("second"), 0600)
			}
			got := r.Run(context.Background(), scenario)
			want := 1
			if mode == "change" {
				want = 2
			}
			if calls != want || opens != want || waits != want-1 || !source.closed || got.Attempts.Started != want || got.Summary.Started != 1 || (got.Err != nil) != (mode != "change") || got.Diverged {
				t.Fatalf("calls=%d opens=%d waits=%d result=%+v", calls, opens, waits, got)
			}
			if mode != "change" && !errors.Is(got.Err, cause) {
				t.Fatalf("cause lost: %v", got.Err)
			}
		})
	}
}

func TestPollingExternalFilesRereadAndPreparationMetrics(t *testing.T) {
	for _, mode := range []string{"change", "missing", "render"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "body")
			if err := os.WriteFile(path, []byte(`{"value":1}`), 0600); err != nil {
				t.Fatal(err)
			}
			scenario := pollingScenario(t, "POST http://example.test/job\n[Options]\nretry: 2\nretry-interval: 0\nfile,body;\nHTTP 200\nGET http://example.test/next\nHTTP 200\n")
			scenario.OriginDir = dir
			calls := 0
			r := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				if calls == 1 {
					return Response{Status: 503}, nil
				}
				if calls == 2 && string(req.Body) != `{"value":2}` {
					t.Fatalf("body=%q", req.Body)
				}
				return Response{Status: 200}, nil
			}), nil)
			r.wait = func(context.Context, time.Duration) error {
				if mode == "missing" {
					return os.Remove(path)
				}
				data := `{"value":2}`
				if mode == "render" {
					data = `{"value":"{{missing}}"}`
				}
				return os.WriteFile(path, []byte(data), 0600)
			}
			got := r.RunWithOptions(context.Background(), scenario, Options{RenderFiles: mode == "render"})
			if mode == "change" {
				if got.Err != nil || got.Diverged || calls != 3 || got.Summary.Started != 2 {
					t.Fatalf("result=%+v calls=%d", got, calls)
				}
			} else {
				if got.Err == nil || got.Diverged || calls != 1 || got.Attempts != (AttemptSummary{1, 1, 1}) || got.Summary != (Summary{2, 1, 1, 0, 0, 1}) || got.Entries[0].Attempts != 1 {
					t.Fatalf("result=%+v calls=%d", got, calls)
				}
			}
		})
	}
}
