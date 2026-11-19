package transport

import (
	"context"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMultipartCompatibleContentTypeParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kind, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || kind != "multipart/form-data" || params["charset"] != "utf-8" || params["boundary"] == "" {
			t.Errorf("Content-Type=%q err=%v", r.Header.Get("Content-Type"), err)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := io.Copy(io.Discard, part); err != nil {
				t.Error(err)
			}
		}
		w.WriteHeader(201)
	}))
	defer server.Close()
	request := uploadRequest(server.URL, strings.NewReader("payload"), 7)
	request.Headers[0].Value = "multipart/form-data; charset=utf-8"
	response, err := NewNetHTTPExecutor().Execute(context.Background(), request)
	if err != nil || response.Status != 201 {
		t.Fatalf("response=%+v err=%v", response, err)
	}
}
