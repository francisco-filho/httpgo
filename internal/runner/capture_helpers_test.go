package runner

import (
	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

// Direct extraction tests explicitly confirm their batch; production confirms
// only after all assertions and the final cancellation barrier.
func captureResponse(session template.Session, declarations []parser.Capture, response Response) *CaptureError {
	batch, failure := extractCaptures(session, declarations, response)
	if failure != nil {
		return failure
	}
	if err := session.Publish(batch); err != nil {
		return captureBatchFailure(session, declarations, err)
	}
	return nil
}
