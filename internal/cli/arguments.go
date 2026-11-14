package cli

import (
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type arguments struct {
	file               string
	variables, secrets []string
}

func parseArguments(args []string) (arguments, error) {
	var result arguments
	invalid := len(args) == 0 || args[0] != "test"
	for i := 1; i < len(args); i++ {
		arg := args[i]
		name, value, inline := strings.Cut(arg, "=")
		switch name {
		case "--var", "--secret":
			if !inline {
				if i+1 == len(args) {
					invalid = true
					continue
				}
				i++
				value = args[i]
			}
			if name == "--var" {
				result.variables = append(result.variables, value)
			} else {
				result.secrets = append(result.secrets, value)
			}
		default:
			if strings.HasPrefix(arg, "-") || result.file != "" {
				invalid = true
			} else {
				result.file = arg
			}
		}
	}
	if invalid || result.file == "" {
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
		// Escape variants protect diagnostics such as quoted filenames as well as
		// already encoded static paths. Provenance protects interpolated URLs first.
		quoted := strconv.Quote(value)
		values = append(values, value, quoted[1:len(quoted)-1], url.PathEscape(value), url.QueryEscape(value))
	}
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	pairs := make([]string, 0, 2*len(values))
	for _, value := range values {
		pairs = append(pairs, value, "[oculto]")
	}
	replacer := strings.NewReplacer(pairs...)
	return replacer.Replace
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
