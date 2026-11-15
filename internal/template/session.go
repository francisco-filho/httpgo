package template

import (
	"encoding/json"
	"strings"
)

// Session owns response-derived JSON values for exactly one execution.
type Session interface {
	Render(string) (Text, error)
	RenderJSON([]byte) ([]byte, error)
	Publish([]Capture) error
	ProtectCapture(name string, value json.RawMessage)
	ProtectValue(string)
	Sanitize(string) string
}

// Capture carries one complete JSON value, never a CLI literal with text fallback.
type Capture struct {
	Name  string
	Value json.RawMessage
}

// CaptureValueError identifies invalid JSON without exposing its content.
type CaptureValueError struct{ Index int }

func (e *CaptureValueError) Error() string { return "valor de captura não contém JSON válido" }

// ConflictError identifies the declaration within a publication batch.
type ConflictError struct{ Index int }

func (e *ConflictError) Error() string {
	return "nome de captura inválido, repetido ou em conflito ancestral/descendente"
}

func NamesConflict(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+".") || strings.HasPrefix(b, a+".")
}

// NewSession shares immutable initial values and secret declarations only.
func (c *Context) NewSession() Session {
	return &Context{values: c.values, secrets: c.secrets, captures: make(map[string]variable)}
}

func (c *Context) lookup(name string) (variable, bool) {
	if v, ok := c.values[name]; ok {
		return v, true
	}
	v, ok := c.captures[name]
	return v, ok
}

// ProtectValue registers sensitive response metadata for this execution only.
func (c *Context) ProtectValue(value string) {
	c.protected = append(c.protected, SecretVariants(value)...)
}

// ProtectCapture remembers a discovered secret even if another declaration fails.
func (c *Context) ProtectCapture(name string, value json.RawMessage) {
	if c.secrets[name] && json.Valid(value) {
		c.protected = append(c.protected, SecretVariants(string(value))...)
	}
}

// Publish is atomic: no capture becomes resolvable if any name conflicts.
func (c *Context) Publish(batch []Capture) error {
	for _, capture := range batch {
		c.ProtectCapture(capture.Name, capture.Value)
	}
	values := make([]variable, len(batch))
	for i, capture := range batch {
		if !json.Valid(capture.Value) {
			return &CaptureValueError{Index: i}
		}
		values[i] = parseVariable(string(capture.Value))
		if !ValidName(capture.Name) {
			return &ConflictError{Index: i}
		}
		for existing := range c.values {
			if NamesConflict(capture.Name, existing) {
				return &ConflictError{Index: i}
			}
		}
		for existing := range c.captures {
			if NamesConflict(capture.Name, existing) {
				return &ConflictError{Index: i}
			}
		}
		for _, other := range batch[:i] {
			if NamesConflict(capture.Name, other.Name) {
				return &ConflictError{Index: i}
			}
		}
	}
	if c.captures == nil {
		c.captures = make(map[string]variable)
	}
	for i, capture := range batch {
		c.captures[capture.Name] = values[i]
	}
	return nil
}
