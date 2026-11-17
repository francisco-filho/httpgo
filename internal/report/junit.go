// Package report serializes execution results without executing or evaluating tests.
package report

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"time"

	"capimgrosso.com/httpgo/internal/suite"
)

type junitSuite struct {
	XMLName  xml.Name    `xml:"testsuite"`
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Errors   int         `xml:"errors,attr"`
	Skipped  int         `xml:"skipped,attr"`
	Time     string      `xml:"time,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitProblem `xml:"failure,omitempty"`
	Error     *junitProblem `xml:"error,omitempty"`
	Skipped   *junitProblem `xml:"skipped,omitempty"`
}

type junitProblem struct {
	Message string `xml:"message,attr"`
	Detail  string `xml:",chardata"`
}

// WriteJUnit consumes already classified suite results. Diagnostics are optional,
// protected presentation text in the same order as result.Files; no operational
// requests, responses or sessions are retained. The caller must sanitize names
// and diagnostics before calling, and owns opening and closing the writer.
func WriteJUnit(writer io.Writer, result suite.Result, diagnostics []string) error {
	document := junitSuite{Name: "httpgo", Tests: len(result.Files), Time: seconds(result.Summary.Duration)}
	for i, file := range result.Files {
		current := junitCase{Name: file.Name, Classname: "httpgo", Time: seconds(file.Duration)}
		detail := ""
		if i < len(diagnostics) {
			detail = diagnostics[i]
		}
		problem := &junitProblem{Message: string(file.State), Detail: detail}
		switch file.State {
		case suite.Passed:
		case suite.Failed:
			document.Failures++
			current.Failure = problem
		case suite.Errored:
			document.Errors++
			current.Error = problem
		case suite.Skipped:
			document.Skipped++
			current.Skipped = problem
		default:
			return fmt.Errorf("estado de arquivo desconhecido no relatório JUnit")
		}
		document.Cases = append(document.Cases, current)
	}
	// encoding/xml buffers output, but does not detect a broken Writer returning
	// n < len(p) with nil error. Enforce io.Writer's contract at that boundary.
	output := completeWriter{writer}
	if _, err := output.Write([]byte(xml.Header)); err != nil {
		return fmt.Errorf("escrever cabeçalho JUnit: %w", err)
	}
	encoder := xml.NewEncoder(output)
	encoder.Indent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return fmt.Errorf("serializar JUnit: %w", err)
	}
	return nil
}

func seconds(duration time.Duration) string {
	return strconv.FormatFloat(max(0, duration.Seconds()), 'f', 9, 64)
}

type completeWriter struct{ io.Writer }

func (w completeWriter) Write(data []byte) (int, error) {
	n, err := w.Writer.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return n, err
}
