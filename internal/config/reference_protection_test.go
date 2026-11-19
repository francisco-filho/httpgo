package config

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

func TestProcessReferenceProtectionBeforeMissingDiagnostic(t *testing.T) {
	for _, missingFirst := range []bool{false, true} {
		missing, available := "zMissing", "aAvailable"
		if missingFirst {
			missing, available = available, missing
		}
		s := referenceSettings(t, "variables: {"+missing+": '${ABSENT}', "+available+": '${AVAILABLE}'}")
		const value = "fiction-value\"雪\n"
		var calls []string
		s.lookupEnv = func(name string) (string, bool) { calls = append(calls, name); return value, name == "AVAILABLE" }
		resolved, err := s.Resolve(context.Background(), nil)
		if err == nil || len(calls) != 2 {
			t.Fatalf("calls=%v err=%v", calls, err)
		}
		if renderer, err := resolved.Compose(nil, nil); renderer != nil || err == nil {
			t.Fatal("partial context was published")
		}
		protect := resolved.Protection(nil, nil)
		for _, variant := range template.SecretVariants(value) {
			if variant != "" && strings.Contains(protect(variant), variant) {
				t.Fatal("available variant leaked before missing diagnostic")
			}
		}
	}
}

func TestProcessReferenceUnionProtectsEverySubstitute(t *testing.T) {
	s, err := environmentSettings(t, "version: 1\nvariables: {token: base-fiction, other: '${DISCARDED}'}\nsecrets: [future]", "variables: {token: '${EFFECTIVE}', other: env-fiction}\nsecrets: [extra]")
	if err != nil {
		t.Fatal(err)
	}
	s.lookupEnv = func(string) (string, bool) { t.Fatal("queried discarded reference"); return "", false }
	assignments := []string{`token="cli-fiction\n雪"`, "other=override-fiction"}
	resolved, err := s.Resolve(context.Background(), assignments)
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := resolved.Compose(assignments, []string{"cliFuture"})
	if err != nil {
		t.Fatal(err)
	}
	session := renderer.NewSession()
	for _, name := range []string{"token", "other", "future", "extra", "cliFuture"} {
		if !session.IsSecret(name) {
			t.Fatalf("lost secret mark %s", name)
		}
	}
	for _, protect := range []func(string) string{resolved.Protection(assignments, nil), renderer.Sanitize, session.Sanitize} {
		for _, value := range []string{"base-fiction", "env-fiction", "cli-fiction\n雪", `cli-fiction\n雪`, "override-fiction"} {
			if strings.Contains(protect(value), value) {
				t.Fatal("substitute leaked")
			}
		}
	}
	captured, _ := json.Marshal("captured-fiction")
	if err := session.Publish([]template.Capture{{Name: "future", Value: captured}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(session.Sanitize("captured-fiction"), "captured-fiction") || renderer.Sanitize("captured-fiction") != "captured-fiction" {
		t.Fatal("capture protection not isolated to session")
	}
}

func TestProcessReferenceProtectionOnMergeConflict(t *testing.T) {
	s, err := environmentSettings(t, "version: 1\nvariables: {token: '${DISCARDED}', shape: 1}", "variables: {token: '${AVAILABLE}', shape: {child: true}}")
	if err == nil {
		t.Fatal("accepted structural conflict")
	}
	var calls []string
	s.lookupEnv = func(name string) (string, bool) {
		calls = append(calls, name)
		return "available-fiction", name == "AVAILABLE"
	}
	resolved, err := s.Resolve(context.Background(), nil)
	if err != nil || len(calls) != 1 || calls[0] != "AVAILABLE" || resolved.Protection(nil, nil)("available-fiction") != "[oculto]" {
		t.Fatalf("calls=%v err=%v", calls, err)
	}
}

func TestProcessReferenceLookupIsolation(t *testing.T) {
	s := referenceSettings(t, "variables: {token: '${TOKEN}'}")
	value := "first-fiction"
	calls := 0
	s.lookupEnv = func(string) (string, bool) { calls++; return value, true }
	first, err := s.Compose(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	value = "second-fiction"
	second, err := s.Compose(nil, nil)
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	for _, tc := range []struct {
		renderer   *template.Context
		own, other string
	}{{first, "first-fiction", "second-fiction"}, {second, "second-fiction", "first-fiction"}} {
		if tc.renderer.Sanitize(tc.own) != "[oculto]" || tc.renderer.Sanitize(tc.other) != tc.other {
			t.Fatal("resolution protection leaked across invocations")
		}
	}
}
