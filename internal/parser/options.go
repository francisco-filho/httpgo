package parser

import (
	"strconv"
	"strings"
	"time"
)

// EntryOptions is local to an entry. Nil interval means the default, not zero.
type EntryOptions struct {
	Variables     []OptionVariable
	Retry         int
	RetryInterval *time.Duration
	Position      Position
}

func (o EntryOptions) Interval() time.Duration {
	if o.RetryInterval == nil {
		return time.Second
	}
	return *o.RetryInterval
}

func parseOption(text string, pos Position, options *EntryOptions, seen map[string]bool) error {
	name, raw, colon := strings.Cut(text, ":")
	fail := func(category, detail string) error { return &Error{Position: pos, Category: category, Detail: detail} }
	if !colon || strings.TrimSpace(name) != name || name == "" {
		return fail("sintaxe inválida", "opção exige nome: inteiro não negativo")
	}
	if name == "variable" {
		declaration, err := parseOptionVariable(raw, pos)
		if err != nil {
			return err
		}
		options.Variables = append(options.Variables, declaration)
		seen[name] = true
		return nil
	}
	if name != "retry" && name != "retry-interval" {
		return fail("construção não suportada", "opção não suportada em [Options]; somente variable, retry e retry-interval são aceitos")
	}
	if seen[name] {
		return fail("sintaxe inválida", "opção "+name+" duplicada")
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.IndexFunc(raw, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return fail("sintaxe inválida", "opção "+name+" exige inteiro não negativo estático")
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	limit := uint64(^uint(0)>>1) - 1 // Include the initial attempt without overflowing int.
	if name == "retry-interval" {
		limit = uint64((1<<63 - 1) / time.Millisecond)
	}
	if err != nil || value > limit {
		return fail("sintaxe inválida", "opção "+name+" fora do limite representável")
	}
	seen[name] = true
	options.Position = pos
	if name == "retry" {
		options.Retry = int(value)
	} else {
		interval := time.Duration(value) * time.Millisecond
		options.RetryInterval = &interval
	}
	return nil
}
