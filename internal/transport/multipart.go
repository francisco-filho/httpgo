package transport

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"
	"sync"

	"capimgrosso.com/httpgo/internal/runner"
)

// uploadReader has no producer goroutine. Close synchronizes with net/http's
// asynchronous request writer before the runner closes or rewinds the source.
type uploadReader struct {
	mu      sync.Mutex
	reader  io.Reader
	done    chan struct{}
	closed  bool
	readErr error
}

func (r *uploadReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return 0, io.ErrClosedPipe
	}
	n, err := r.reader.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		r.readErr = err
	}
	return n, err
}
func (r *uploadReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.closed = true
		close(r.done)
	}
	return nil
}
func (r *uploadReader) finished() error {
	<-r.done
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.readErr
}

// Preserve compatible declared parameters; boundary is always transport-owned.
func multipartContentType(request runner.Request, boundary string) (string, error) {
	params := make(map[string]string)
	for _, header := range request.Headers {
		if !strings.EqualFold(header.Name, "Content-Type") {
			continue
		}
		kind, declared, err := mime.ParseMediaType(header.Value)
		if err != nil {
			return "", fmt.Errorf("Content-Type multipart inválido: %w", err)
		}
		if _, manual := declared["boundary"]; manual || kind != "multipart/form-data" {
			return "", fmt.Errorf("Content-Type multipart incompatível ou com boundary manual")
		}
		params = declared
	}
	params["boundary"] = boundary
	return mime.FormatMediaType("multipart/form-data", params), nil
}

func multipartReader(body *runner.MultipartBody, boundary string) (*uploadReader, int64, error) {
	if _, err := body.Source.Seek(0, io.SeekStart); err != nil {
		return nil, 0, fmt.Errorf("rebobinar upload: %w", err)
	}
	var framing bytes.Buffer
	writer := multipart.NewWriter(&framing)
	if err := writer.SetBoundary(boundary); err != nil {
		return nil, 0, err
	}
	var readers []io.Reader
	var length int64
	for _, part := range body.Parts {
		if !part.File {
			if err := writer.WriteField(part.Name, part.Value); err != nil {
				return nil, 0, err
			}
			continue
		}
		header := make(textproto.MIMEHeader)
		// FormatMediaType uses RFC 2231 for Unicode, which multipart receivers do
		// not universally decode. Hurl sends quoted UTF-8 in these parameters.
		quote := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
		header.Set("Content-Disposition", `form-data; name="`+quote.Replace(part.Name)+`"; filename="`+quote.Replace(part.Filename)+`"`)
		header.Set("Content-Type", part.MIME)
		if _, err := writer.CreatePart(header); err != nil {
			return nil, 0, err
		}
		prefix := framing.String()
		length += int64(len(prefix))
		readers = append(readers, strings.NewReader(prefix), body.Source)
		framing.Reset()
	}
	if err := writer.Close(); err != nil {
		return nil, 0, err
	}
	suffix := framing.String()
	length += int64(len(suffix)) + body.Size
	readers = append(readers, strings.NewReader(suffix))
	return &uploadReader{reader: io.MultiReader(readers...), done: make(chan struct{})}, length, nil
}
