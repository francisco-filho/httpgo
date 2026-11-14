package parser

import (
	"strings"
	"testing"
)

func TestRenderedURLValidation(t *testing.T) {
	for _, raw := range []string{"HTTP://example.test/a%2Fb?q=a/b", "http://example.test/#", "https://user:password@example.test:443/path?x=%20#fragment", "http://[::1]:8080/", "https://user%3apassword@example.test/a%2fb#"} {
		if err := ValidateURL(raw); err != nil {
			t.Errorf("ValidateURL(%q)=%v", raw, err)
		}
	}
	for _, raw := range []string{"/relative", "ftp://example.test", "http://:80", "http://example.test:65536", "http://example.test/%", "http://example.test/%a", "http://example.test/?x=%zz", "http://example.test/é", "http://example.test/not encoded", "http://example.test/\x7f", "http://example.test/{value}"} {
		if err := ValidateURL(raw); err == nil {
			t.Errorf("ValidateURL(%q) accepted invalid final URL", raw)
		}
	}
}

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
