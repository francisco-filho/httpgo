// Package parser validates the complete US-01 subset before execution.
package parser

import (
	"bufio"
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

type Entry struct {
	URL              string
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
	scenario := Scenario{File: file}
	reader := bufio.NewReader(input)
	for line := 1; ; line++ {
		raw, readErr := reader.ReadString('\n')
		pos := Position{File: file, Line: line, Column: 1}
		if readErr != nil && readErr != io.EOF {
			return Scenario{}, &Error{Position: pos, Category: "erro de leitura", Detail: "não foi possível ler o arquivo", cause: readErr}
		}
		if err := parseLine(&scenario, strings.TrimSuffix(strings.TrimSuffix(raw, "\n"), "\r"), pos); err != nil {
			return Scenario{}, err
		}
		if readErr == io.EOF {
			break
		}
	}
	if len(scenario.Entries) == 0 {
		return Scenario{}, &Error{Position: Position{file, 1, 1}, Category: "sintaxe inválida", Detail: "o arquivo precisa conter ao menos um GET"}
	}
	return scenario, nil
}

func parseLine(scenario *Scenario, raw string, pos Position) error {
	text := strings.TrimLeft(raw, " \t")
	pos.Column += len(raw) - len(text)
	fail := func(category, detail string) error {
		return &Error{Position: pos, Category: category, Detail: detail}
	}
	if !utf8.ValidString(raw) || strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsControl(r) && r != '\t' }) {
		return fail("sintaxe inválida", "caractere inválido")
	}
	if text == "" || strings.HasPrefix(text, "#") {
		return nil
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return fail("sintaxe inválida", "espaçamento inválido")
	}
	switch fields[0] {
	case "GET":
		if len(fields) != 2 {
			return fail("sintaxe inválida", "GET exige somente uma URL absoluta HTTP/HTTPS; comentários apenas em linha própria")
		}
		if strings.Contains(fields[1], "{{") || strings.Contains(fields[1], "}}") {
			return fail("construção não suportada", "variáveis")
		}
		u, err := parseURL(fields[1])
		if err != nil {
			pos.Column += strings.Index(text, fields[1])
			return fail("sintaxe inválida", "URL absoluta HTTP/HTTPS inválida")
		}
		scenario.Entries = append(scenario.Entries, Entry{URL: u.String(), Position: pos})
		return nil
	case "HTTP":
		if len(fields) != 2 {
			return fail("sintaxe inválida", "HTTP exige somente um status de três dígitos")
		}
		if len(scenario.Entries) == 0 || scenario.Entries[len(scenario.Entries)-1].ExpectedStatus != nil {
			return fail("sintaxe inválida", "expectativa HTTP sem GET disponível")
		}
		status, err := strconv.Atoi(fields[1])
		if err != nil || len(fields[1]) != 3 || fields[1][0] < '1' || fields[1][0] > '5' || status < 100 || status > 599 {
			return fail("sintaxe inválida", "status deve ter três dígitos entre 100 e 599")
		}
		entry := &scenario.Entries[len(scenario.Entries)-1]
		entry.ExpectedStatus, entry.ExpectedPosition = &status, pos
		return nil
	default:
		if construction := unsupported(fields[0]); construction != "" {
			return fail("construção não suportada", construction)
		}
		return fail("sintaxe inválida", "instrução desconhecida; use GET ou HTTP")
	}
}

func unsupported(token string) string {
	switch token {
	case "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "CONNECT", "TRACE":
		return "método " + token
	case "[Captures]", "[Asserts]", "[Options]", "[QueryStringParams]", "[FormParams]", "[MultipartFormData]", "[Cookies]", "[BasicAuth]":
		return "seção " + token
	default:
		switch {
		case strings.HasPrefix(token, "HTTP/"):
			return "versão HTTP na resposta esperada"
		case strings.HasSuffix(token, ":"):
			return "header"
		case strings.HasPrefix(token, "file,"):
			return "body externo"
		case strings.HasPrefix(token, "{") || strings.HasPrefix(token, "\"") || strings.HasPrefix(token, "```"):
			return "body ou variável"
		case strings.HasPrefix(token, "["):
			return "seção ou body"
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
