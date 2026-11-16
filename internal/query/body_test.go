package query

import (
	"strings"
	"testing"
)

func TestBodyText(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, body, want string
		invalid                       bool
	}{
		{"default UTF8", "", "ação 雪 �", "ação 雪 �", false},
		{"UTF8 declared", "text/plain; charset=UTF-8", "ação", "ação", false},
		{"empty", "", "", "", false},
		{"latin1 alias", "text/plain; charset=ISO-8859-1", "a\xe7\xe3o", "ação", false},
		{"windows label", "text/plain; charset=windows-1252", "\x80", "€", false},
		{"quoted charset", `text/plain; charset="utf-8"`, "雪", "雪", false},
		{"UTF16LE", "text/plain; charset=utf-16le", "a\x00\xfd\xff", "a�", false},
		{"UTF16BE", "text/plain; charset=utf-16be", "\x00a\xff\xfd", "a�", false},
		{"UTF16LE BOM literal replacement", "text/plain; charset=utf-16le", "\xff\xfe\xfd\xff", "\ufeff�", false},
		{"UTF16 surrogate pair", "text/plain; charset=utf-16be", "\xd8\x3d\xde\x00", "😀", false},
		{"UTF16 unmatched low", "text/plain; charset=utf-16be", "\xdc\x00", "", true},
		{"UTF16 unmatched high", "text/plain; charset=utf-16be", "\xd8\x00\x00a", "", true},
		{"GB18030 literal replacement", "text/plain; charset=gb18030", "\x84\x31\xa4\x37", "�", false},
		{"unknown", "text/plain; charset=private-label", "body-private", "", true},
		{"unknown empty", "text/plain; charset=private-label", "", "", true},
		{"invalid UTF8", "", "body-private\xff", "", true},
		{"truncated UTF8", "text/plain; charset=utf8", "\xe2\x82", "", true},
		{"invalid shiftjis", "text/plain; charset=shift_jis", "\x82", "", true},
		{"invalid UTF16", "text/plain; charset=utf-16le", "\x00\xd8", "", true},
		{"truncated UTF16", "text/plain; charset=utf-16be", "\x00", "", true},
		{"invalid MIME", "text/plain; charset=private-label; charset=utf-8", "body-private", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			headers := []HeaderValue(nil)
			if tc.contentType != "" {
				headers = []HeaderValue{{"Content-Type", tc.contentType}}
			}
			got, err := BodyText([]byte(tc.body), headers)
			if (err != nil) != tc.invalid || got != tc.want {
				t.Fatalf("got=%q err=%v want=%q invalid=%v", got, err, tc.want, tc.invalid)
			}
			if err != nil && (strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), tc.body) && tc.body != "") {
				t.Fatalf("raw input in error: %v", err)
			}
		})
	}
}

func TestBodyTextUsesFirstContentType(t *testing.T) {
	got, err := BodyText([]byte("\xe9"), []HeaderValue{{"content-type", "text/plain; charset=iso-8859-1"}, {"Content-Type", "text/plain; charset=utf-8"}})
	if got != "é" || err != nil {
		t.Fatalf("text=%q err=%v", got, err)
	}
}
