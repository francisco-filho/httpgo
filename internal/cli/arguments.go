package cli

import (
	"fmt"
	"io"
	"strings"

	"capimgrosso.com/httpgo/internal/config"
	"capimgrosso.com/httpgo/internal/template"
)

type arguments struct {
	renderFiles        bool
	files              []string
	config             string
	environment        string
	variables, secrets []string
}

func parseArguments(args []string) (arguments, error) {
	var result arguments
	invalid := len(args) == 0 || args[0] != "test"
	environmentSeen := false
	environmentIssue := ""
	for i := 1; i < len(args); i++ {
		arg := args[i]
		name, value, inline := strings.Cut(arg, "=")
		switch name {
		case "--render-files":
			if inline {
				invalid = true
			}
			result.renderFiles = true
		case "--config", "--env", "--var", "--secret":
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

// executionProtection is local to ExecuteContext. The runner still owns session
// creation; its factory publishes only the sanitizer to the output boundary.
type executionProtection struct {
	initial func(string) string
	session template.Session
}

func (p *executionProtection) sanitize(text string) string {
	if p.session != nil {
		text = p.session.Sanitize(text)
	}
	return p.initial(text)
}

type sessionRenderer struct {
	context    *template.Context
	protection *executionProtection
}

func (r sessionRenderer) NewSession() template.Session {
	session := r.context.NewSession()
	r.protection.session = session
	return session
}

type protectedWriter struct {
	writer  io.Writer
	protect func(string) string
}

func (w protectedWriter) Write(data []byte) (int, error) {
	safe := w.protect(string(data))
	n, err := io.WriteString(w.writer, safe)
	if err != nil {
		return 0, err
	}
	if n != len(safe) {
		return 0, io.ErrShortWrite
	}
	return len(data), nil
}
