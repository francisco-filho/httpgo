package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

// MultipartBody is operational metadata only, never a report value.
// Runner owns Source. Execute must finish all reads before returning.
type MultipartBody struct {
	Parts  []MultipartPart
	Source io.ReadSeeker
	Size   int64
}

type MultipartPart struct {
	Name, Value, Filename, MIME string
	File                        bool
}

type uploadFile interface {
	io.ReadSeekCloser
	Stat() (os.FileInfo, error)
}

func openUpload(path string) (uploadFile, error) { return os.Open(path) }

// renderMultipart resolves textual fields and static metadata once per entry.
// Values follow the same template rules as headers and URLs.
func renderMultipart(session template.Session, entry parser.Entry, headers []parser.Header) (*MultipartBody, *RenderError) {
	form := entry.Multipart
	body := &MultipartBody{}
	for _, declaration := range form.Parts {
		part := MultipartPart{Name: declaration.Name, File: declaration.File != nil, MIME: declaration.MIME}
		if declaration.File != nil {
			part.Filename = filepath.Base(declaration.File.Path)
			if !utf8.ValidString(part.Filename) || strings.ContainsFunc(part.Filename, unicode.IsControl) {
				return nil, &RenderError{Position: declaration.File.Position, Detail: "filename de multipart inválido"}
			}
			if part.MIME == "" {
				part.MIME = multipartMIME(part.Filename)
			}
		} else {
			text, err := session.Render(declaration.Value)
			if err != nil {
				return nil, renderFailure(declaration.ValuePosition, declaration.Value, err)
			}
			part.Value = text.Value
			if text.Value != text.Display {
				session.ProtectValue(text.Value)
			}
		}
		body.Parts = append(body.Parts, part)
	}
	for i, header := range headers {
		if strings.EqualFold(header.Name, "Content-Type") {
			if err := parser.ValidateMultipartContentType(header.Value); err != nil {
				pos := form.Position
				if i < len(entry.HeaderPositions) {
					pos = entry.HeaderPositions[i]
				}
				return nil, &RenderError{Position: pos, Detail: err.Error(), cause: err}
			}
		}
	}
	return body, nil
}

// openMultipart opens the single file part per attempt and keeps the rendered
// text fields from the entry preparation. Execute must consume the returned file.
func (r *Runner) openMultipart(ctx context.Context, entry parser.Entry, origin string, request *Request) (io.Closer, error) {
	var reference *parser.FileBody
	for _, declaration := range entry.Multipart.Parts {
		if declaration.File != nil {
			reference = declaration.File
		}
	}
	if reference == nil {
		return nil, &RenderError{Position: entry.Multipart.Position, Detail: "multipart exige exatamente um arquivo"}
	}
	if err := ctx.Err(); err != nil {
		return nil, fileFailure(reference.Position, err)
	}
	path := reference.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(origin, path)
	}
	file, err := r.openUpload(path)
	if err != nil {
		return nil, fileFailure(reference.Position, err)
	}
	info, err := file.Stat()
	if err == nil && info.IsDir() {
		err = errPayloadDirectory
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return nil, fileFailure(reference.Position, errors.Join(err, file.Close()))
	}
	request.Multipart = &MultipartBody{Parts: slices.Clone(request.Multipart.Parts), Source: file, Size: info.Size()}
	return file, nil
}
