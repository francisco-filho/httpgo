// Package parser validates the complete supported subset before execution.
package parser

import (
	"bytes"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"capimgrosso.com/httpgo/internal/template"
)

type Position struct {
	File   string
	Line   int
	Column int
}

type Header struct{ Name, Value string }

type Entry struct {
	Method           string
	URL              string
	Headers          []Header
	Body             []byte
	BodyPosition     Position
	BodyPlan         template.JSONPlan
	Position         Position
	ExpectedStatus   *int
	ExpectedPosition Position
	URLPosition      Position
	HeaderPositions  []Position
}

type Scenario struct {
	File    string
	Entries []Entry
}

// Error contains only controlled diagnostic text, never input tokens.
type Error struct {
	Position Position
	Category string
	Detail   string
	cause    error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%q:%d:%d: %s: %s", e.Position.File, e.Position.Line, e.Position.Column, e.Category, e.Detail)
}
func (e *Error) Unwrap() error { return e.cause }

func Parse(file string, input io.Reader) (Scenario, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return Scenario{}, &Error{Position: Position{file, 1, 1}, Category: "erro de leitura", Detail: "não foi possível ler o arquivo", cause: err}
	}
	scenario := Scenario{File: file}
	for offset := 0; offset < len(data); {
		end := bytes.IndexByte(data[offset:], '\n')
		if end < 0 {
			end = len(data)
		} else {
			end += offset
		}
		raw := strings.TrimSuffix(string(data[offset:end]), "\r")
		text := strings.TrimLeft(raw, " \t")
		start := offset + len(raw) - len(text)
		pos := position(file, data, start)
		fail := func(category, detail string) (Scenario, error) {
			return Scenario{}, &Error{Position: pos, Category: category, Detail: detail}
		}
		if !utf8.ValidString(raw) || strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsControl(r) && r != '\t' }) {
			return fail("sintaxe inválida", "caractere inválido em instrução ou header")
		}
		if text == "" || strings.HasPrefix(text, "#") {
			offset = end + 1
			continue
		}
		fields := strings.Fields(text)
		if len(fields) == 0 {
			return fail("sintaxe inválida", "espaçamento inválido")
		}
		var entry *Entry
		if len(scenario.Entries) > 0 {
			entry = &scenario.Entries[len(scenario.Entries)-1]
		}
		switch fields[0] {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
			token := strings.TrimLeft(text[len(fields[0]):], " \t")
			urlPos := pos
			urlPos.Column += len(text) - len(token)
			refs, err := template.Parse(token)
			if err != nil {
				pos = templatePosition(urlPos, err)
				return fail("sintaxe inválida", err.Error())
			}
			// Only placeholder interiors can contain whitespace in the request URL.
			masked, last := "", 0
			for _, ref := range refs {
				masked += token[last:ref.Start] + "x"
				last = ref.End
			}
			masked += token[last:]
			if token == "" || strings.ContainsFunc(masked, unicode.IsSpace) {
				return fail("sintaxe inválida", "método exige somente uma URL absoluta HTTP/HTTPS; comentários apenas em linha própria")
			}
			if len(refs) == 0 {
				u, err := parseURL(token)
				if err != nil {
					pos = urlPos
					return fail("sintaxe inválida", "URL absoluta HTTP/HTTPS inválida")
				}
				token = u.String()
			}
			scenario.Entries = append(scenario.Entries, Entry{Method: fields[0], URL: token, Position: pos, URLPosition: urlPos})
		case "HTTP":
			if len(fields) != 2 {
				return fail("sintaxe inválida", "HTTP exige somente um status de três dígitos")
			}
			if entry == nil || entry.ExpectedStatus != nil {
				return fail("sintaxe inválida", "expectativa HTTP sem requisição disponível")
			}
			status, err := strconv.Atoi(fields[1])
			if err != nil || len(fields[1]) != 3 || fields[1][0] < '1' || fields[1][0] > '5' || status < 100 || status > 599 {
				return fail("sintaxe inválida", "status deve ter três dígitos entre 100 e 599")
			}
			entry.ExpectedStatus, entry.ExpectedPosition = &status, pos
		default:
			if construction := unsupported(fields[0]); construction != "" {
				return fail("construção não suportada", construction)
			}
			if isJSONStart(text[0]) && (!strings.Contains(text, ":") || text[0] == '{' || text[0] == '[' || text[0] == '"') {
				if entry == nil || entry.ExpectedStatus != nil || entry.Body != nil {
					return fail("sintaxe inválida", "body JSON sem requisição disponível ou body duplicado")
				}
				if entry.Method == "GET" {
					return fail("construção não suportada", "body em GET")
				}
				plan, err := template.ParseJSON(data[start:])
				if err != nil {
					if located, ok := err.(*template.Error); ok {
						pos = position(file, data, start+located.Offset)
					}
					category := "sintaxe inválida"
					if strings.Contains(err.Error(), "não são suportadas") {
						category = "construção não suportada"
					}
					return fail(category, err.Error())
				}
				finish := start + plan.End
				body := data[start:finish]
				// Only whitespace may follow a value on its last line. Never absorb
				// a second value, an expectation, a comment or the following request.
				tailEnd := bytes.IndexByte(data[finish:], '\n')
				if tailEnd < 0 {
					tailEnd = len(data)
				} else {
					tailEnd += finish
				}
				if strings.Trim(string(data[finish:tailEnd]), " \t\r") != "" {
					pos = position(file, data, finish)
					return fail("sintaxe inválida", "conteúdo após o valor JSON; use uma nova linha")
				}
				entry.Body = bytes.Clone(body)
				entry.BodyPosition = position(file, data, start)
				entry.BodyPlan = plan
				offset = tailEnd + 1
				continue
			}
			if strings.Contains(text, ":") {
				if entry == nil || entry.ExpectedStatus != nil || entry.Body != nil {
					return fail("sintaxe inválida", "header fora da requisição ou após body")
				}
				name, value, _ := strings.Cut(text, ":")
				if hasTemplate(name) {
					return fail("construção não suportada", "variáveis no nome de header")
				}
				if !validHeaderName(name) || !validHeaderValue(value) {
					return fail("sintaxe inválida", "nome ou valor de header inválido")
				}
				valuePos := pos
				valuePos.Column += len(name) + 1 + len(value) - len(strings.TrimLeft(value, " \t"))
				value = strings.Trim(value, " \t")
				refs, err := template.Parse(value)
				if err != nil {
					pos = templatePosition(valuePos, err)
					return fail("sintaxe inválida", err.Error())
				}
				entry.HeaderPositions = append(entry.HeaderPositions, valuePos)
				switch strings.ToLower(name) {
				case "host":
					host := strings.Trim(value, " \t")
					if len(refs) == 0 && ValidateHeader(name, host) != nil {
						return fail("sintaxe inválida", "valor de Host inválido")
					}
					for _, header := range entry.Headers {
						if strings.EqualFold(header.Name, name) {
							return fail("sintaxe inválida", "Host duplicado")
						}
					}
					entry.Headers = append(entry.Headers, Header{Name: name, Value: host})
				case "content-length", "transfer-encoding", "trailer":
					return fail("construção não suportada", "header de enquadramento HTTP; o transporte calcula o tamanho do body")
				default:
					entry.Headers = append(entry.Headers, Header{Name: name, Value: strings.Trim(value, " \t")})
				}
			} else {
				return fail("sintaxe inválida", "instrução ou header inválido")
			}
		}
		offset = end + 1
	}
	if len(scenario.Entries) == 0 {
		return Scenario{}, &Error{Position: Position{file, 1, 1}, Category: "sintaxe inválida", Detail: "o arquivo precisa conter ao menos uma requisição"}
	}
	return scenario, nil
}

func position(file string, data []byte, offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(data) {
		offset = len(data)
	}
	prefix := data[:offset]
	return Position{file, bytes.Count(prefix, []byte{'\n'}) + 1, offset - bytes.LastIndexByte(prefix, '\n')}
}

func templatePosition(pos Position, err error) Position {
	if located, ok := err.(*template.Error); ok {
		pos.Column += located.Offset
	}
	return pos
}

func hasTemplate(s string) bool { return strings.Contains(s, "{{") || strings.Contains(s, "}}") }
func isJSONStart(c byte) bool   { return strings.ContainsRune("{[\"-0123456789tfn", rune(c)) }
func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range []byte(name) {
		valid := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", rune(c))
		if !valid {
			return false
		}
	}
	return true
}
func validHeaderValue(value string) bool {
	for _, c := range []byte(value) {
		if c == 127 || (c < 32 && c != '\t') {
			return false
		}
	}
	return true
}

func unsupported(token string) string {
	switch token {
	case "HEAD", "OPTIONS", "CONNECT", "TRACE":
		return "método " + token
	case "[Captures]", "[Asserts]", "[Options]", "[QueryStringParams]", "[FormParams]", "[MultipartFormData]", "[Cookies]", "[BasicAuth]":
		return "seção " + token
	default:
		switch {
		case strings.HasPrefix(token, "HTTP/"):
			return "versão HTTP na resposta esperada"
		case strings.HasPrefix(token, "file,"):
			return "body externo"
		case strings.HasPrefix(token, "```"):
			return "body binário ou string multilinha"
		case strings.HasPrefix(token, "[") && len(token) > 1 && ((token[1] >= 'A' && token[1] <= 'Z') || (token[1] >= 'a' && token[1] <= 'z' && token[1] != 't' && token[1] != 'f' && token[1] != 'n')):
			return "seção desconhecida"
		default:
			return ""
		}
	}
}

// ValidateURL validates a rendered URL without normalizing or encoding its bytes.
func ValidateURL(raw string) error {
	for _, c := range []byte(raw) {
		if c <= 32 || c >= 127 || strings.ContainsRune("\"<>\\^`{|}", rune(c)) {
			return fmt.Errorf("URL absoluta HTTP/HTTPS inválida; forneça o encoding necessário")
		}
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '%' {
			continue
		}
		if i+2 >= len(raw) || !isHex(raw[i+1]) || !isHex(raw[i+2]) {
			return fmt.Errorf("encoding de URL inválido")
		}
		i += 2
	}
	u, err := parseURL(raw)
	if err != nil {
		return fmt.Errorf("URL absoluta HTTP/HTTPS inválida")
	}
	// Compare the actual request target with the supplied bytes, not URL.String:
	// the latter may legitimately canonicalize percent-encoded userinfo.
	authority := raw[strings.Index(raw, "://")+3:]
	target := ""
	if start := strings.IndexAny(authority, "/?#"); start >= 0 {
		target = authority[start:]
	}
	target, _, _ = strings.Cut(target, "#")
	if target == "" || target[0] == '?' {
		target = "/" + target
	}
	if u.RequestURI() != target {
		return fmt.Errorf("URL exige encoding explícito")
	}
	return nil
}

func isHex(c byte) bool { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }

// ValidateHeader applies the same value and Host rules to static and final values.
func ValidateHeader(name, value string) error {
	if !validHeaderName(name) || !validHeaderValue(value) || !utf8.ValidString(value) {
		return fmt.Errorf("nome ou valor de header inválido")
	}
	if strings.EqualFold(name, "Host") {
		u, err := parseURL("http://" + value)
		if err != nil || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(value, " \t/?#") {
			return fmt.Errorf("valor de Host inválido")
		}
	}
	return nil
}

func parseURL(token string) (*url.URL, error) {
	u, err := url.Parse(token)
	if err != nil {
		return nil, err
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if (u.Scheme != "http" && u.Scheme != "https") || u.Opaque != "" || u.Hostname() == "" || strings.ContainsAny(u.Host, "{}\\") {
		return nil, fmt.Errorf("invalid url")
	}
	if strings.HasPrefix(u.Host, "[") {
		address, err := netip.ParseAddr(u.Hostname())
		if err != nil || !address.Is6() {
			return nil, fmt.Errorf("invalid ipv6")
		}
	}
	if strings.Contains(u.Hostname(), ":") && !strings.HasPrefix(u.Host, "[") {
		return nil, fmt.Errorf("unbracketed ip")
	}
	if strings.HasSuffix(u.Host, ":") {
		return nil, fmt.Errorf("empty port")
	}
	if port := u.Port(); port != "" {
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return nil, err
		}
	}
	return u, nil
}
