package parser

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestEntryOptions(t *testing.T) {
	for _, tc := range []struct {
		input    string
		retry    int
		interval time.Duration
	}{
		{"", 0, time.Second},
		{"[Options]\nretry: 3\n", 3, time.Second},
		{"[Options]\nretry-interval: 0\nretry: 0\n", 0, 0},
		{"[Options]\nretry: 2\nretry-interval: 500\n", 2, 500 * time.Millisecond},
	} {
		t.Run(tc.input, func(t *testing.T) {
			scenario, err := Parse("options.hurl", strings.NewReader("POST http://example.test\n"+tc.input+"{\"ok\":true}\nHTTP 200\nGET http://example.test/next\n"))
			if err != nil {
				t.Fatal(err)
			}
			got := scenario.Entries[0].Options
			if got.Retry != tc.retry || got.Interval() != tc.interval || scenario.Entries[1].Options.Retry != 0 || scenario.Entries[1].Options.Interval() != time.Second {
				t.Fatalf("options=%+v next=%+v", got, scenario.Entries[1].Options)
			}
		})
	}
}

func TestInvalidEntryOptionsBeforeExecution(t *testing.T) {
	maxInt := strconv.FormatUint(uint64(^uint(0)>>1), 10)
	for _, tc := range []struct{ text, category string }{
		{"retry: -1", "sintaxe inválida"}, {"retry: 0.5", "sintaxe inválida"}, {"retry: +1", "sintaxe inválida"},
		{"retry: {{budget}}", "sintaxe inválida"}, {"retry: 18446744073709551616", "sintaxe inválida"}, {"retry: " + maxInt, "sintaxe inválida"},
		{"retry-interval: 9223372036855", "sintaxe inválida"}, {"retry-interval: -1", "sintaxe inválida"}, {"retry-interval: 1.1", "sintaxe inválida"},
		{"retry: 1\nretry: 2", "sintaxe inválida"}, {"retry-interval: 0\nretry-interval: 0", "sintaxe inválida"},
		{"retry: 1 # private", "sintaxe inválida"}, {"retry: 1\n[Options]\nretry: 2", "sintaxe inválida"},
		{"timeout: 5", "construção não suportada"}, {"private-option: private", "construção não suportada"},
	} {
		for _, newline := range []string{"\n", "\r\n"} {
			t.Run(fmt.Sprintf("%q/%q", tc.text, newline), func(t *testing.T) {
				input := "GET http://example.test/first\nHTTP 200\nGET http://example.test/job\n[Options]\n  " + tc.text + "\nHTTP 200\n"
				scenario, err := Parse("options.hurl", strings.NewReader(strings.ReplaceAll(input, "\n", newline)))
				var located *Error
				if !errors.As(err, &located) || located.Category != tc.category || len(scenario.Entries) != 0 || located.Position.Line < 5 || strings.Contains(err.Error(), "private") {
					t.Fatalf("scenario=%+v error=%v", scenario, err)
				}
				if !strings.Contains(tc.text, "\n") && located.Position.Column != 3 {
					t.Fatalf("position=%+v", located.Position)
				}
			})
		}
	}
	for _, input := range []string{"[Options]\nretry: 1", "GET http://example.test\nHTTP 200\n[Options]\nretry: 1", "GET http://example.test\n[Options]", "GET http://example.test\n[Options]\nHTTP 200", "GET http://example.test\n[Options]\nGET http://example.test/next"} {
		if _, err := Parse("options.hurl", strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}
