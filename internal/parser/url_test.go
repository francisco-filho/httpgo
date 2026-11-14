package parser

import (
	"strings"
	"testing"
)

func TestAbsoluteHTTPURLs(t *testing.T) {
	for _, input := range []string{
		"HTTP://example.test:080/health", "https://example.test:443/", "http://[::1]:8080/health", "http://[fe80::1%25eth0]:8080/health",
	} {
		t.Run(input, func(t *testing.T) {
			scenario, err := Parse("url.hurl", strings.NewReader("GET "+input))
			if err != nil || len(scenario.Entries) != 1 {
				t.Fatalf("Parse(%q) = %+v, %v", input, scenario, err)
			}
		})
	}
	if _, err := Parse("url.hurl", strings.NewReader("GET http://[127.0.0.1]/")); err == nil {
		t.Fatal("accepted bracketed IPv4")
	}
}
