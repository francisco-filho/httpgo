package parser

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

// OptionVariable retains every declaration, including overwritten literals, for
// validation, located publication errors and monotonic secret protection.
type OptionVariable struct {
	Name     string
	Value    json.RawMessage
	Position Position
}

func parseOptionVariable(raw string, pos Position) (OptionVariable, error) {
	fail := func() (OptionVariable, error) {
		return OptionVariable{}, &Error{Position: pos, Category: "sintaxe inválida", Detail: "opção variable exige nome simples=literal válido"}
	}
	name, literal, ok := strings.Cut(strings.TrimSpace(raw), "=")
	if !ok || name == "" {
		return fail()
	}
	for i, c := range name {
		valid := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_' || i > 0 && c >= '0' && c <= '9'
		if !valid {
			return fail()
		}
	}
	value, ok := optionLiteral(strings.TrimSpace(literal))
	if !ok {
		return fail()
	}
	return OptionVariable{Name: name, Value: value, Position: pos}, nil
}

// optionLiteral implements the scalar option profile, not CLI JSON inference.
// Compound values and apparent references remain opaque strings. Numbers never
// pass through float64, including decimals and integers beyond 2^53.
func optionLiteral(source string) (json.RawMessage, bool) {
	quoted := strings.HasPrefix(source, `"`)
	text, tail, ok := optionString(source, quoted)
	if !ok || strings.TrimSpace(tail) != "" && !strings.HasPrefix(strings.TrimSpace(tail), "#") {
		return nil, false
	}
	if !quoted {
		switch text {
		case "true", "false", "null":
			return json.RawMessage(text), true
		}
		if text != "" && (text[0] >= '0' && text[0] <= '9' || text[0] == '+' || text[0] == '-') {
			number := strings.TrimPrefix(text, "+")
			if !json.Valid([]byte(number)) || strings.ContainsAny(number, "eE") {
				return nil, false
			}
			return json.RawMessage(number), true
		}
		for _, keyword := range []string{"true", "false", "null"} {
			if strings.HasPrefix(text, keyword) {
				return nil, false
			}
		}
	}
	value, err := json.Marshal(text)
	return value, err == nil
}

// Decode the Hurl option string escapes without expanding templates. Unicode
// escapes use \u{hex}, unlike JSON bodies. A single quote is ordinary text.
func optionString(source string, quoted bool) (string, string, bool) {
	var out strings.Builder
	start := 0
	if quoted {
		start = 1
	}
	for i := start; i < len(source); i++ {
		c := source[i]
		if quoted && c == '"' {
			return out.String(), source[i+1:], true
		}
		if !quoted && c == '#' {
			return strings.TrimRight(out.String(), " \t"), source[i:], true
		}
		if c != '\\' {
			out.WriteByte(c)
			continue
		}
		i++
		if i == len(source) {
			return "", "", false
		}
		switch source[i] {
		case '\\', '#':
			out.WriteByte(source[i])
		case '"':
			if !quoted {
				return "", "", false
			}
			out.WriteByte('"')
		case ':':
			if quoted {
				return "", "", false
			}
			out.WriteByte(':')
		case 'b':
			out.WriteByte('\b')
		case 'f':
			out.WriteByte('\f')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'u':
			if i+1 >= len(source) || source[i+1] != '{' {
				return "", "", false
			}
			end := strings.IndexByte(source[i+2:], '}')
			if end < 1 {
				return "", "", false
			}
			value, err := strconv.ParseUint(source[i+2:i+2+end], 16, 32)
			if err != nil || !utf8.ValidRune(rune(value)) {
				return "", "", false
			}
			out.WriteRune(rune(value))
			i += end + 2
		default:
			return "", "", false
		}
	}
	return out.String(), "", !quoted
}
