package parser

import (
	"fmt"
	"mime"
	"strings"
	"unicode"

	"capimgrosso.com/httpgo/internal/template"
)

// MultipartForm preserves declaration order and source positions.
type MultipartForm struct {
	Parts    []MultipartPart
	Position Position
}

type MultipartPart struct {
	Name          string
	Value         string
	File          *FileBody
	MIME          string
	Position      Position
	ValuePosition Position
}

func parseMultipartPart(text string, pos Position) (MultipartPart, error) {
	fail := func(detail string) (MultipartPart, error) {
		return MultipartPart{}, &Error{Position: pos, Category: "sintaxe inválida", Detail: detail}
	}
	name, value, ok := strings.Cut(text, ":")
	if !ok || name == "" || strings.TrimSpace(name) != name || hasTemplate(name) || strings.ContainsFunc(name, unicode.IsControl) {
		return fail("multipart exige nome estático não vazio e valor separados por :")
	}
	valuePos := pos
	valuePos.Column += len(name) + 1 + len(value) - len(strings.TrimLeft(value, " \t"))
	value = strings.TrimLeft(value, " \t")
	part := MultipartPart{Name: name, Value: value, Position: pos, ValuePosition: valuePos}
	if strings.HasPrefix(value, "file,") {
		path, media, found := strings.Cut(value[5:], ";")
		if !found || path == "" || strings.ContainsAny(path, ",;") || strings.Contains(media, ";") {
			return fail("referência multipart file exige caminho literal e terminador ; seguido de MIME opcional")
		}
		part.File = &FileBody{Path: path, Position: valuePos}
		part.Value = ""
		part.MIME = strings.TrimSpace(media)
		if part.MIME != "" {
			kind, params, err := mime.ParseMediaType(part.MIME)
			if err != nil || len(params) != 0 || !strings.Contains(kind, "/") || hasTemplate(part.MIME) || strings.ContainsFunc(part.MIME, unicode.IsControl) {
				return fail("MIME de multipart inválido; use tipo/subtipo sem parâmetros")
			}
		}
	} else if _, err := template.Parse(value); err != nil {
		return MultipartPart{}, &Error{Position: templatePosition(valuePos, err), Category: "sintaxe inválida", Detail: err.Error()}
	}
	return part, nil
}

// ValidateMultipartContentType refuses all manual parameters, including boundary.
func ValidateMultipartContentType(value string) error {
	kind, params, err := mime.ParseMediaType(value)
	if err != nil || kind != "multipart/form-data" || len(params) != 0 {
		return fmt.Errorf("Content-Type de multipart deve ser multipart/form-data sem boundary ou parâmetros")
	}
	return nil
}

func validateMultipart(entry Entry) error {
	form := entry.Multipart
	if form == nil {
		return nil
	}
	fail := func(pos Position, detail string) error {
		return &Error{Position: pos, Category: "sintaxe inválida", Detail: detail}
	}
	files := 0
	names := make(map[string]bool)
	for _, part := range form.Parts {
		if names[part.Name] {
			return fail(part.Position, "nome de parte multipart repetido")
		}
		names[part.Name] = true
		if part.File != nil {
			files++
		}
	}
	if files != 1 {
		return fail(form.Position, "multipart exige exatamente um arquivo")
	}
	count := 0
	for i, header := range entry.Headers {
		if strings.EqualFold(header.Name, "Content-Type") {
			count++
			refs, _ := template.Parse(header.Value)
			if len(refs) == 0 {
				if err := ValidateMultipartContentType(header.Value); err != nil {
					return fail(entry.HeaderPositions[i], err.Error())
				}
			}
		}
	}
	if count > 1 {
		return fail(form.Position, "Content-Type duplicado em multipart")
	}
	return nil
}
