package template

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// JSONReference records byte intervals in the original template, not rendered JSON.
type JSONReference struct {
	Reference
	InString bool
}

type JSONPlan struct {
	References []JSONReference
	End        int
}

type jsonScanner struct {
	source []byte
	offset int
	refs   []JSONReference
}

func jsonError(offset int, detail string) error { return &Error{offset, detail} }

// ParseJSON recognizes exactly one value prefix, allowing a Hurl instruction on
// the following line. It validates structure independently of variable values.
func ParseJSON(source []byte) (JSONPlan, error) {
	s := jsonScanner{source: source}
	if err := s.value(0); err != nil {
		return JSONPlan{}, err
	}
	body := source[:s.offset]
	if !utf8.Valid(body) {
		return JSONPlan{}, jsonError(0, "body JSON deve usar UTF-8 válido")
	}
	masked := substituteJSON(body, s.refs, func(ref JSONReference) string {
		if ref.InString {
			return "x"
		}
		return "null"
	})
	if !json.Valid(masked) {
		return JSONPlan{}, jsonError(0, "body JSON inválido; declare um único valor JSON")
	}
	return JSONPlan{References: s.refs, End: s.offset}, nil
}

func (s *jsonScanner) space() {
	for s.offset < len(s.source) && strings.ContainsRune(" \t\r\n", rune(s.source[s.offset])) {
		s.offset++
	}
}
func (s *jsonScanner) take(c byte) bool {
	s.space()
	if s.offset < len(s.source) && s.source[s.offset] == c {
		s.offset++
		return true
	}
	return false
}
func (s *jsonScanner) reference(inString bool) error {
	start := s.offset
	close := bytes.Index(s.source[start+2:], []byte("}}"))
	if close < 0 {
		return jsonError(start, "placeholder inválido: fechamento ausente")
	}
	end := start + 2 + close + 2
	refs, err := Parse(string(s.source[start:end]))
	if err != nil {
		return jsonError(start, err.Error())
	}
	s.refs = append(s.refs, JSONReference{Reference: Reference{start, end, refs[0].Name}, InString: inString})
	s.offset = end
	return nil
}
func (s *jsonScanner) quoted(key bool) error {
	start := s.offset
	s.offset++
	for s.offset < len(s.source) {
		switch s.source[s.offset] {
		case '"':
			s.offset++
			return nil
		case '\\':
			s.offset += 2
			continue
		}
		if bytes.HasPrefix(s.source[s.offset:], []byte("{{")) {
			if key {
				return jsonError(s.offset, "variáveis em chave JSON não são suportadas")
			}
			if err := s.reference(true); err != nil {
				return err
			}
			continue
		}
		s.offset++
	}
	return jsonError(start, "string JSON inválida ou não terminada")
}
func (s *jsonScanner) value(depth int) error {
	s.space()
	start := s.offset
	if depth > 10000 || start >= len(s.source) {
		return jsonError(start, "body JSON inválido ou aninhamento excessivo")
	}
	if bytes.HasPrefix(s.source[start:], []byte("{{")) {
		return s.reference(false)
	}
	switch s.source[start] {
	case '"':
		return s.quoted(false)
	case '{':
		s.offset++
		if s.take('}') {
			return nil
		}
		for {
			s.space()
			if bytes.HasPrefix(s.source[s.offset:], []byte("{{")) {
				return jsonError(s.offset, "variáveis em chave JSON não são suportadas")
			}
			if s.offset >= len(s.source) || s.source[s.offset] != '"' {
				return jsonError(s.offset, "objeto JSON exige chave estática entre aspas")
			}
			if err := s.quoted(true); err != nil {
				return err
			}
			if !s.take(':') {
				return jsonError(s.offset, "objeto JSON exige dois-pontos após chave")
			}
			if err := s.value(depth + 1); err != nil {
				return err
			}
			if s.take('}') {
				return nil
			}
			if !s.take(',') {
				return jsonError(s.offset, "objeto JSON exige vírgula ou fechamento")
			}
		}
	case '[':
		s.offset++
		if s.take(']') {
			return nil
		}
		for {
			if err := s.value(depth + 1); err != nil {
				return err
			}
			if s.take(']') {
				return nil
			}
			if !s.take(',') {
				return jsonError(s.offset, "array JSON exige vírgula ou fechamento")
			}
		}
	default:
		for s.offset < len(s.source) && !strings.ContainsRune(" \t\r\n,]}", rune(s.source[s.offset])) {
			s.offset++
		}
		if s.offset == start || !json.Valid(s.source[start:s.offset]) {
			return jsonError(start, "valor JSON inválido")
		}
		return nil
	}
}

func substituteJSON(source []byte, refs []JSONReference, replacement func(JSONReference) string) []byte {
	var out bytes.Buffer
	last := 0
	for _, ref := range refs {
		out.Write(source[last:ref.Start])
		out.WriteString(replacement(ref))
		last = ref.End
	}
	out.Write(source[last:])
	return out.Bytes()
}

// RenderJSON inserts once, escaping only string content. Static JSON is copied
// unchanged; diagnostics contain neither payloads nor encoding/json causes.
func (c *Context) RenderJSON(source []byte) ([]byte, error) {
	plan, err := ParseJSON(source)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(source[plan.End:])) != 0 {
		return nil, jsonError(plan.End, "conteúdo após o valor JSON")
	}
	replacements := make(map[int]string, len(plan.References))
	for _, ref := range plan.References {
		v, ok := c.values[ref.Name]
		if !ok {
			return nil, jsonError(ref.Start, "variável ausente; forneça seu valor com --var")
		}
		if ref.InString {
			if v.composite {
				return nil, jsonError(ref.Start, "array ou objeto exige posição de valor JSON completo")
			}
			encoded, _ := json.Marshal(v.text)
			replacements[ref.Start] = string(encoded[1 : len(encoded)-1])
		} else {
			replacements[ref.Start] = string(v.raw)
		}
	}
	rendered := substituteJSON(source, plan.References, func(ref JSONReference) string { return replacements[ref.Start] })
	if !json.Valid(rendered) {
		return nil, jsonError(0, "body JSON renderizado inválido")
	}
	return rendered, nil
}
