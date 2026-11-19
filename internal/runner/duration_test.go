package runner

import (
	"context"
	"testing"
	"time"
)

func TestLogicalEntryDurationIncludesPreparationRetriesAndWait(t *testing.T) {
	for _, backwards := range []bool{false, true} {
		t.Run(map[bool]string{false: "elapsed", true: "backwards"}[backwards], func(t *testing.T) {
			scenario := pollingScenario(t, "POST http://example.test/job\nfile,payload.json;\n[Options]\nretry: 1\nretry-interval: 100\nHTTP 200\nGET http://example.test/not-reached\nHTTP 200")
			now := time.Date(2026, 11, 17, 19, 30, 45, 0, time.UTC)
			calls, reports, reads := 0, 0, 0
			r := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				now = now.Add(20 * time.Millisecond)
				return Response{Status: 503}, nil
			}), func(entry EntryResult) error {
				reports++
				want := 160 * time.Millisecond
				if backwards {
					want = 0
				}
				if entry.Duration != want || entry.Attempts != 2 {
					t.Fatalf("entry=%+v want duration=%s", entry, want)
				}
				now = now.Add(time.Hour) // Presentation must not count.
				return nil
			})
			r.readFile = func(context.Context, string) ([]byte, error) {
				reads++
				now = now.Add(10 * time.Millisecond)
				return []byte(`{}`), nil
			}
			r.wait = func(_ context.Context, d time.Duration) error { now = now.Add(d); return nil }
			clockCalls := 0
			r.now = func() time.Time {
				clockCalls++
				if backwards && clockCalls == 2 {
					return now.Add(-time.Hour)
				}
				return now
			}
			result := r.Run(context.Background(), scenario)
			if calls != 2 || reads != 2 || reports != 1 || clockCalls != 2 || len(result.Entries) != 1 || result.Summary.NotStarted != 1 || result.Attempts.Started != 2 {
				t.Fatalf("calls=%d reads=%d reports=%d clocks=%d result=%+v", calls, reads, reports, clockCalls, result)
			}
		})
	}
}
