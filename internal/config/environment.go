package config

import (
	"encoding/json"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Both explicit and implicit map paths survive flattening, including empty maps.
// Positions belong to their source, not to the eventual effective document.
type variablePath struct {
	mapping, explicit bool
	file              string
	node              *yaml.Node
}

func declarePath(file string, node *yaml.Node, name string, mapping bool, paths map[string]variablePath) error {
	if previous, ok := paths[name]; ok && (previous.explicit || !mapping || !previous.mapping) {
		return failure(file, node, "caminhos de variáveis repetidos ou em conflito", nil)
	}
	segments := strings.Split(name, ".")
	for i := 1; i < len(segments); i++ {
		parent := strings.Join(segments[:i], ".")
		if previous, ok := paths[parent]; ok {
			if !previous.mapping {
				return failure(file, node, "caminhos de variáveis repetidos ou em conflito", nil)
			}
		} else {
			paths[parent] = variablePath{mapping: true, file: file, node: node}
		}
	}
	paths[name] = variablePath{mapping: mapping, explicit: true, file: file, node: node}
	return nil
}

func mergeEnvironment(base, environment Settings) (Settings, error) {
	// Collect every valid source and every mark before detecting merge conflicts.
	// A later mark also protects old values overridden in another source.
	base.Secrets = append(append([]string(nil), base.Secrets...), environment.Secrets...)
	base.sources = []map[string]json.RawMessage{base.Variables, environment.Variables}
	for name, path := range environment.paths {
		if previous, ok := base.paths[name]; ok && previous.mapping != path.mapping {
			return base, failure(path.file, path.node, "conflito entre mapa de caminhos e valor de variável", nil)
		}
	}
	values := make(map[string]json.RawMessage, len(base.Variables)+len(environment.Variables))
	for name, value := range base.Variables {
		values[name] = value
	}
	for name, value := range environment.Variables {
		values[name] = value
	}
	paths := make(map[string]variablePath, len(base.paths)+len(environment.paths))
	for name, path := range base.paths {
		paths[name] = path
	}
	for name, path := range environment.paths {
		paths[name] = path
	}
	base.Variables, base.paths = values, paths
	if environment.timeoutSet {
		base.Timeout = environment.Timeout
	}
	return base, nil
}

func (s Settings) cliPathConflict(name string) bool {
	for existing, path := range s.paths {
		if existing == name && path.mapping || strings.HasPrefix(existing, name+".") || !path.mapping && strings.HasPrefix(name, existing+".") {
			return true
		}
	}
	return false
}
