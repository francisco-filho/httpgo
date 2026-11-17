package report

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/suite"
)

var update = flag.Bool("update", false, "update JUnit golden file")

func TestJUnitResults(t *testing.T) {
	result := suite.Result{
		Files: []suite.FileResult{
			{Name: "tests/01-ação & \"aspas\" <ok>.hurl", State: suite.Passed, Duration: 1250 * time.Millisecond},
			{Name: "tests/02-failure.hurl", State: suite.Failed, Duration: 2 * time.Second},
			{Name: "tests/03-error.hurl", State: suite.Errored, Duration: 3 * time.Millisecond},
			{Name: "tests/04-skipped.hurl", State: suite.Skipped},
		},
		Summary: suite.Summary{Duration: 4 * time.Second}, Code: 2,
	}
	diagnostics := []string{"", "\"tests/02-failure.hurl\":4:1: assertion: esperado \"ação & <x>\", recebido \"outro\"", "\"tests/03-error.hurl\":1:1: falha de transporte", ""}
	before := append([]suite.FileResult(nil), result.Files...)
	var out bytes.Buffer
	if err := WriteJUnit(&out, result, diagnostics); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, result.Files) || result.Code != 2 {
		t.Fatal("rendering changed execution results")
	}
	var got junitSuite
	if err := xml.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid XML: %v\n%s", err, out.String())
	}
	if got.Tests != 4 || got.Failures != 1 || got.Errors != 1 || got.Skipped != 1 || got.Time != "4.000000000" || len(got.Cases) != 4 {
		t.Fatalf("suite=%+v", got)
	}
	for i, tc := range got.Cases {
		if tc.Name != result.Files[i].Name || tc.Classname != "httpgo" || tc.Time != seconds(result.Files[i].Duration) {
			t.Fatalf("testcase %d=%+v", i, tc)
		}
	}
	if got.Cases[0].Failure != nil || got.Cases[0].Error != nil || got.Cases[0].Skipped != nil || got.Cases[1].Failure.Detail != diagnostics[1] || got.Cases[2].Error.Detail != diagnostics[2] || got.Cases[3].Skipped == nil {
		t.Fatalf("incorrect classification: %+v", got.Cases)
	}
	golden := filepath.Join("..", "..", "testdata", "report", "junit.xml")
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, out.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("JUnit differs from golden (-update to regenerate):\n%s", out.String())
	}
}

type brokenWriter struct {
	calls, allowed int
	cause          error
}

func (w *brokenWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls > w.allowed {
		return 0, w.cause
	}
	return len(p), nil
}

func TestJUnitWriteErrors(t *testing.T) {
	cause := errors.New("writer failed")
	for _, allowed := range []int{0, 1} {
		for _, short := range []bool{false, true} {
			writer := &brokenWriter{allowed: allowed, cause: cause}
			want := cause
			if short {
				writer.cause, want = nil, io.ErrShortWrite
			}
			err := WriteJUnit(writer, suite.Result{}, nil)
			if !errors.Is(err, want) || writer.calls != allowed+1 {
				t.Fatalf("allowed=%d short=%t err=%v writes=%d", allowed, short, err, writer.calls)
			}
		}
	}
}

func TestJUnitUnknownStateAndInvalidXMLCharacters(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJUnit(&out, suite.Result{Files: []suite.FileResult{{State: "unknown"}}}, nil); err == nil || out.Len() != 0 {
		t.Fatalf("unknown state: err=%v output=%q", err, out.String())
	}
	result := suite.Result{Files: []suite.FileResult{{Name: "controls\x00\xff", State: suite.Errored, Duration: -time.Second}}}
	if err := WriteJUnit(&out, result, []string{"detail\x01\xff"}); err != nil {
		t.Fatal(err)
	}
	var got junitSuite
	if err := xml.Unmarshal(out.Bytes(), &got); err != nil || !strings.Contains(got.Cases[0].Name, "\uFFFD") || got.Cases[0].Time != "0.000000000" {
		t.Fatalf("invalid character replacement: err=%v output=%q", err, out.String())
	}
}
