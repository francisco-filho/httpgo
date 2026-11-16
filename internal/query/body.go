package query

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"mime"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/htmlindex"
)

// BodyText decodes only the final response bytes. It never includes input or
// library errors in diagnostics, and never sniffs content or normalizes text.
func BodyText(body []byte, headers []HeaderValue) (string, error) {
	label := "utf-8"
	if contentType, present := Header(headers, "Content-Type"); present && contentType != "" {
		_, parameters, err := mime.ParseMediaType(contentType)
		if err != nil {
			return "", fmt.Errorf("Content-Type inválido para decodificação textual")
		}
		if charset, present := parameters["charset"]; present {
			label = strings.TrimSpace(charset)
		}
	}
	encoding, err := htmlindex.Get(label)
	if err != nil {
		return "", fmt.Errorf("charset de resposta desconhecido")
	}
	name, err := htmlindex.Name(encoding)
	if err != nil || name == "replacement" {
		return "", fmt.Errorf("charset de resposta não suportado")
	}
	if name == "utf-8" {
		if !utf8.Valid(body) {
			return "", fmt.Errorf("sequência inválida no charset da resposta")
		}
		return string(body), nil
	}
	unicode16 := name == "utf-16le" || name == "utf-16be"
	if unicode16 && !validUTF16(body, name == "utf-16le") {
		return "", fmt.Errorf("sequência inválida no charset da resposta")
	}
	text, err := encoding.NewDecoder().Bytes(body)
	if err != nil {
		return "", fmt.Errorf("sequência inválida no charset da resposta")
	}
	// x/text decoders can silently substitute malformed sequences. A literal
	// U+FFFD is valid only if it can round-trip, unlike a substituted rune.
	if !unicode16 && bytes.Contains(text, []byte("\uFFFD")) {
		original, err := encoding.NewEncoder().Bytes(text)
		if err != nil || !bytes.Equal(original, body) {
			return "", fmt.Errorf("sequência inválida no charset da resposta")
		}
	}
	return string(text), nil
}

func validUTF16(body []byte, littleEndian bool) bool {
	if len(body)%2 != 0 {
		return false
	}
	var order binary.ByteOrder = binary.BigEndian
	if littleEndian {
		order = binary.LittleEndian
	}
	for i := 0; i < len(body); i += 2 {
		value := order.Uint16(body[i:])
		switch {
		case value >= 0xd800 && value <= 0xdbff:
			i += 2
			if i >= len(body) {
				return false
			}
			low := order.Uint16(body[i:])
			if low < 0xdc00 || low > 0xdfff {
				return false
			}
		case value >= 0xdc00 && value <= 0xdfff:
			return false
		}
	}
	return true
}
