package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"capimgrosso.com/httpgo/internal/config"
	"capimgrosso.com/httpgo/internal/template"
)

type arguments struct {
	renderFiles        bool
	renderAsserts      bool
	failFast           bool
	files              []string
	config             string
	environment        string
	output, outputFile string
	variables, secrets []string
}

func parseArguments(args []string) (arguments, error) {
	var result arguments
	invalid := len(args) == 0 || args[0] != "test"
	environmentSeen := false
	outputSeen, outputFileSeen := false, false
	environmentIssue := ""
	for i := 1; i < len(args); i++ {
		arg := args[i]
		name, value, inline := strings.Cut(arg, "=")
		switch name {
		case "--fail-fast":
			if inline {
				invalid = true
			}
			result.failFast = true
		case "--render-asserts":
			if inline {
				invalid = true
			}
			result.renderAsserts = true
		case "--render-files":
			if inline {
				invalid = true
			}
			result.renderFiles = true
		case "--config", "--env", "--var", "--secret", "--output", "--output-file":
			if !inline {
				if i+1 == len(args) {
					invalid = true
					if name == "--env" {
						environmentIssue = "--env exige um nome de ambiente"
					}
					continue
				}
				i++
				value = args[i]
			}
			switch name {
			case "--output":
				if outputSeen || value != "junit" {
					invalid = true
				}
				outputSeen, result.output = true, value
			case "--output-file":
				if outputFileSeen || value == "" {
					invalid = true
				}
				outputFileSeen, result.outputFile = true, value
			case "--config":
				if value == "" || result.config != "" {
					invalid = true
				}
				result.config = value
			case "--env":
				if environmentSeen {
					environmentIssue = "--env pode ser informado somente uma vez, mesmo com nome igual"
				} else if !config.ValidEnvironmentName(value) {
					environmentIssue = "nome de ambiente inválido em --env; use nome não vazio com letras ASCII, números, _ ou -"
				}
				environmentSeen = true
				result.environment = value
			case "--var":
				result.variables = append(result.variables, value)
			case "--secret":
				result.secrets = append(result.secrets, value)
			}
		default:
			if strings.HasPrefix(arg, "-") || arg == "" {
				invalid = true
			} else {
				result.files = append(result.files, arg)
			}
		}
	}
	if environmentIssue != "" {
		return result, fmt.Errorf("%s; use httpgo --help", environmentIssue)
	}
	if outputSeen != outputFileSeen {
		return result, fmt.Errorf("JUnit exige --output junit e --output-file <caminho>; use httpgo --help")
	}
	if invalid || len(result.files) == 0 {
		return result, fmt.Errorf("uso inválido ou flag não suportada; use httpgo test <arquivo> ou httpgo --help")
	}
	return result, nil
}

// Scan declarations independently of argument validity/order: even errors that
// precede context construction must not expose a later protected value.
func argumentProtection(args []string) func(string) string {
	var assignments, secrets []string
	for i, arg := range args {
		name, value, inline := strings.Cut(arg, "=")
		if !inline && i+1 < len(args) {
			value = args[i+1]
		}
		switch name {
		case "--var":
			assignments = append(assignments, value)
		case "--secret":
			secrets = append(secrets, value)
		}
	}
	protected := make(map[string]bool)
	for _, name := range secrets {
		protected[name] = true
	}
	var values []string
	for _, assignment := range assignments {
		name, value, ok := strings.Cut(assignment, "=")
		if !ok || value == "" || !protected[name] {
			continue
		}
		values = append(values, template.SecretVariants(value)...)
	}
	return template.Protector(values)
}

// executionProtection lives for one invocation and retains only deduplicated
// redaction variants, never sessions, captures or operational variable contexts.
type executionProtection struct {
	initial func(string) string
	values  map[string]bool
	protect func(string) string
}

func (p *executionProtection) remember(values []string) {
	if p.values == nil {
		p.values = make(map[string]bool)
	}
	changed := false
	for _, value := range values {
		if value != "" && !p.values[value] {
			p.values[value] = true
			changed = true
		}
	}
	if changed {
		all := make([]string, 0, len(p.values))
		for value := range p.values {
			all = append(all, value)
		}
		p.protect = template.Protector(all)
	}
}

func (p *executionProtection) sanitize(text string) string {
	if p.protect != nil {
		text = p.protect(text)
	}
	return p.initial(text)
}

type sessionRenderer struct {
	context    *template.Context
	protection *executionProtection
}

func (r sessionRenderer) NewSession() template.Session {
	session := r.context.NewSession()
	r.protection.remember(session.ProtectionValues())
	return invocationSession{Session: session, protection: r.protection}
}

// The wrapper shares only protection, not resolution, with other file sessions.
type invocationSession struct {
	template.Session
	protection *executionProtection
}

func (s invocationSession) Sanitize(text string) string {
	return s.protection.sanitize(s.Session.Sanitize(text))
}

func (s invocationSession) ProtectValue(value string) {
	s.Session.ProtectValue(value)
	s.protection.remember(s.ProtectionValues())
}

func (s invocationSession) ProtectCapture(name string, value json.RawMessage) {
	s.Session.ProtectCapture(name, value)
	s.protection.remember(s.ProtectionValues())
}

func (s invocationSession) MarkSecret(name string) {
	s.Session.MarkSecret(name)
	s.protection.remember(s.ProtectionValues())
}

func (s invocationSession) Stage(batch []template.Capture) (template.Session, error) {
	view, err := s.Session.Stage(batch)
	s.protection.remember(s.ProtectionValues())
	if err != nil {
		return nil, err
	}
	return invocationSession{Session: view, protection: s.protection}, nil
}

func (s invocationSession) Publish(batch []template.Capture) error {
	err := s.Session.Publish(batch)
	s.protection.remember(s.ProtectionValues())
	return err
}

type protectedWriter struct {
	writer  io.Writer
	protect func(string) string
	failure *error
}

func (w protectedWriter) Write(data []byte) (int, error) {
	if w.failure != nil && *w.failure != nil {
		return 0, *w.failure
	}
	safe := w.protect(string(data))
	n, err := io.WriteString(w.writer, safe)
	if err == nil && n != len(safe) {
		err = io.ErrShortWrite
	}
	if err != nil {
		if w.failure != nil {
			*w.failure = err
		}
		return 0, err
	}
	return len(data), nil
}
