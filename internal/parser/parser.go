// Package parser validates the complete supported subset before execution.
package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
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
	Position         Position
	ExpectedStatus   *int
	ExpectedPosition Position
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
			if len(fields) != 2 {
				return fail("sintaxe inválida", "método exige somente uma URL absoluta HTTP/HTTPS; comentários apenas em linha própria")
			}
			if hasTemplate(fields[1]) {
				return fail("construção não suportada", "variáveis")
			}
			u, err := parseURL(fields[1])
			if err != nil {
				pos.Column += strings.Index(text, fields[1])
				return fail("sintaxe inválida", "URL absoluta HTTP/HTTPS inválida")
			}
			scenario.Entries = append(scenario.Entries, Entry{Method: fields[0], URL: u.String(), Position: pos})
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
				decoder := json.NewDecoder(bytes.NewReader(data[start:]))
				var value json.RawMessage
				if err := decoder.Decode(&value); err != nil {
					if syntax, ok := err.(*json.SyntaxError); ok {
						pos = position(file, data, start+int(syntax.Offset)-1)
					}
					return fail("sintaxe inválida", "body JSON inválido; declare um único valor JSON")
				}
				finish := start + int(decoder.InputOffset())
				body := data[start:finish]
				if !utf8.Valid(body) {
					return fail("sintaxe inválida", "body JSON deve usar UTF-8 válido")
				}
				if bytes.Contains(body, []byte("{{")) {
					return fail("construção não suportada", "variáveis no body")
				}
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
				offset = tailEnd + 1
				continue
			}
			if strings.Contains(text, ":") {
				if entry == nil || entry.ExpectedStatus != nil || entry.Body != nil {
					return fail("sintaxe inválida", "header fora da requisição ou após body")
				}
				name, value, _ := strings.Cut(text, ":")
				if !validHeaderName(name) || !validHeaderValue(value) {
					return fail("sintaxe inválida", "nome ou valor de header inválido")
				}
				if hasTemplate(value) {
					return fail("construção não suportada", "variáveis no header")
				}
				switch strings.ToLower(name) {
				case "host":
					host := strings.Trim(value, " \t")
					u, err := parseURL("http://" + host)
					if err != nil || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(host, " \t/?#") {
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
		case strings.HasPrefix(token, "{{"):
			return "variáveis"
		case strings.HasPrefix(token, "```"):
			return "body binário ou string multilinha"
		case strings.HasPrefix(token, "[") && len(token) > 1 && ((token[1] >= 'A' && token[1] <= 'Z') || (token[1] >= 'a' && token[1] <= 'z' && token[1] != 't' && token[1] != 'f' && token[1] != 'n')):
			return "seção desconhecida"
		default:
			return ""
		}
	}
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
