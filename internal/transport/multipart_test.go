package transport

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestMultipartWire(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "PATCH"} {
		for _, payload := range []string{"", "\x00\xff{{missing}}"} {
			for _, fields := range []bool{false, true} {
				t.Run(method+payload+map[bool]string{true: "fields", false: "file-only"}[fields], func(t *testing.T) {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != method {
							t.Errorf("method=%s", r.Method)
						}
						kind, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
						if err != nil || kind != "multipart/form-data" || params["boundary"] == "" {
							t.Errorf("header=%q err=%v", r.Header.Get("Content-Type"), err)
							return
						}
						reader := multipart.NewReader(r.Body, params["boundary"])
						count := 0
						for {
							part, err := reader.NextPart()
							if err == io.EOF {
								break
							}
							if err != nil {
								t.Error(err)
								return
							}
							data, err := io.ReadAll(part)
							if err != nil {
								t.Error(err)
							}
							switch part.FormName() {
							case "document":
								if part.FileName() != "olá mundo.pdf" || part.Header.Get("Content-Type") != "application/pdf" || string(data) != payload {
									t.Errorf("part=%v data=%q", part.Header, data)
								}
							case "description":
								if string(data) != "Documento" {
									t.Errorf("text=%q", data)
								}
							default:
								t.Errorf("unexpected part=%v", part.Header)
							}
							count++
						}
						want := 1
						if fields {
							want++
						}
						if count != want {
							t.Errorf("parts=%d want=%d", count, want)
						}
						w.WriteHeader(201)
					}))
					defer server.Close()
					parts := []runner.MultipartPart{}
					if fields {
						parts = append(parts, runner.MultipartPart{Name: "description", Value: "Documento"})
					}
					parts = append(parts, runner.MultipartPart{Name: "document", File: true, Filename: "olá mundo.pdf", MIME: "application/pdf"})
					response, err := NewNetHTTPExecutor().Execute(context.Background(), runner.Request{Method: method, URL: server.URL, Multipart: &runner.MultipartBody{Parts: parts, Source: strings.NewReader(payload), Size: int64(len(payload))}})
					if err != nil || response.Status != 201 {
						t.Fatalf("response=%+v err=%v", response, err)
					}
				})
			}
		}
	}
}

// A virtual file makes a whole-file buffer visible without allocating a test
// payload or allowing the receiver to retain it. No global memory limit changes.
type largeUpload struct {
	offset, size int64
	reads        int
}

func (f *largeUpload) Read(p []byte) (int, error) {
	if f.offset == f.size {
		return 0, io.EOF
	}
	n := int(min(int64(len(p)), f.size-f.offset))
	clear(p[:n])
	f.offset += int64(n)
	f.reads++
	return n, nil
}
func (f *largeUpload) Seek(offset int64, whence int) (int64, error) {
	if offset != 0 || whence != io.SeekStart {
		return 0, io.ErrUnexpectedEOF
	}
	f.offset = 0
	return 0, nil
}

func TestMultipartLargeUploadMemory(t *testing.T) {
	const size = 64 << 20
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Error(err)
			return
		}
		n, err := io.CopyBuffer(io.Discard, part, make([]byte, 32<<10))
		if err != nil || n != size {
			t.Errorf("bytes=%d err=%v", n, err)
		}
		w.WriteHeader(201)
	}))
	defer server.Close()
	source := &largeUpload{size: size}
	body := &runner.MultipartBody{Parts: []runner.MultipartPart{{Name: "file", File: true, Filename: "large.bin", MIME: "application/octet-stream"}}, Source: source, Size: size}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	response, err := NewNetHTTPExecutor().Execute(context.Background(), runner.Request{Method: "POST", URL: server.URL, Multipart: body})
	runtime.ReadMemStats(&after)
	if err != nil || response.Status != 201 || source.reads < 2 || after.TotalAlloc-before.TotalAlloc > 16<<20 {
		t.Fatalf("status=%d err=%v reads=%d allocated=%d", response.Status, err, source.reads, after.TotalAlloc-before.TotalAlloc)
	}
}

func TestMultipartFramingOrder(t *testing.T) {
	body := &runner.MultipartBody{Parts: []runner.MultipartPart{{Name: "a", Value: "before"}, {Name: "file", File: true, Filename: "a.bin", MIME: "application/octet-stream"}, {Name: "z", Value: "after"}}, Source: bytes.NewReader([]byte("payload")), Size: 7}
	reader, length, err := multipartReader(body, "fixed")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || int64(len(data)) != length || bytes.Index(data, []byte("before")) >= bytes.Index(data, []byte("payload")) || bytes.Index(data, []byte("payload")) >= bytes.Index(data, []byte("after")) {
		t.Fatalf("length=%d data=%q err=%v", length, data, err)
	}
}
