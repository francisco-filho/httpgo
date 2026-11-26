package template

import (
	"encoding/json"
	"maps"
	"strings"
)

// Session owns response-derived JSON values for exactly one execution.
type Session interface {
	Render(string) (Text, error)
	RenderJSON([]byte) ([]byte, error)
	Stage([]Capture) (Session, error) // Provisional resolution; only protection is shared.
	Publish([]Capture) error
	ProtectCapture(name string, value json.RawMessage)
	ProtectValue(string)
	MarkSecret(string)
	IsSecret(string) bool
	Sanitize(string) string
	ProtectionValues() []string // Detached known variants, never a session or variable context.
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

// ReservedError identifies a capture using a reserved built-in function name.
type ReservedError struct{ Index int }

func (e *ReservedError) Error() string {
	return "nome de captura reservado para função embutida (newDate, newUuid)"
}
func NamesConflict(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+".") || strings.HasPrefix(b, a+".")
}

// NewSession shares immutable initial values and the function clock; secret
// declarations are session-local.
func (c *Context) NewSession() Session {
	return &Context{values: c.values, secrets: maps.Clone(c.secrets), captures: make(map[string]variable), protected: append([]string(nil), c.protected...), now: c.now}
}
func (c *Context) lookup(name string) (variable, bool) {
	if v, ok := c.captures[name]; ok {
		return v, true
	}
	v, ok := c.values[name]
	return v, ok
}

// ProtectValue registers sensitive response metadata for this execution only.
func (c *Context) ProtectValue(value string) {
	c.protected = append(c.protected, SecretVariants(value)...)
}

// MarkSecret accumulates a declaration without resolving or publishing a value.
func (c *Context) MarkSecret(name string) {
	if c.secrets == nil {
		c.secrets = make(map[string]bool)
	}
	c.secrets[name] = true
	// A newly derived secret may replace an already confirmed value.
	if value, ok := c.lookup(name); ok {
		c.ProtectValue(value.original)
	}
}

// IsSecret inspects only the declaration, without resolving or publishing a value.
func (c *Context) IsSecret(name string) bool { return c.secrets[name] }

// ProtectCapture remembers a discovered secret even if another declaration fails.
func (c *Context) ProtectCapture(name string, value json.RawMessage) {
	if c.secrets[name] && json.Valid(value) {
		c.ProtectValue(string(value))
	}
}

// captureValues validates everything before any value becomes resolvable.
func (c *Context) captureValues(batch []Capture) ([]variable, error) {
	for _, capture := range batch {
		c.ProtectCapture(capture.Name, capture.Value)
	}
	values := make([]variable, len(batch))
	for i, capture := range batch {
		if !json.Valid(capture.Value) {
			return nil, &CaptureValueError{Index: i}
		}
		values[i] = parseVariable(string(capture.Value))
		if !ValidName(capture.Name) {
			return nil, &ConflictError{Index: i}
		}
		if ReservedFunction(capture.Name) {
			return nil, &ReservedError{Index: i}
		}
		for existing := range c.values {
			if capture.Name != existing && NamesConflict(capture.Name, existing) {
				return nil, &ConflictError{Index: i}
			}
		}
		for existing := range c.captures {
			if capture.Name != existing && NamesConflict(capture.Name, existing) {
				return nil, &ConflictError{Index: i}
			}
		}
		for _, other := range batch[:i] {
			if NamesConflict(capture.Name, other.Name) {
				return nil, &ConflictError{Index: i}
			}
		}
	}
	return values, nil
}

// provisional resolves a detached overlay while forwarding all protection to
// the confirmed session. Discarding it cannot roll back secret knowledge.
type provisional struct {
	Session
	view *Context
}

func (p *provisional) Render(source string) (Text, error) {
	text, err := p.view.Render(source)
	text.Display = p.Sanitize(text.Display)
	return text, err
}
func (p *provisional) RenderJSON(source []byte) ([]byte, error) { return p.view.RenderJSON(source) }

func (p *provisional) MarkSecret(name string) {
	p.Session.MarkSecret(name)
	if value, ok := p.view.lookup(name); ok {
		p.ProtectValue(value.original)
	}
}

// Stage never changes confirmed resolution or the immutable initial context.
func (c *Context) Stage(batch []Capture) (Session, error) {
	values, err := c.captureValues(batch)
	if err != nil {
		return nil, err
	}
	captures := maps.Clone(c.captures)
	if captures == nil {
		captures = make(map[string]variable)
	}
	for i, capture := range batch {
		captures[capture.Name] = values[i]
	}
	return &provisional{Session: c, view: &Context{values: c.values, captures: captures, secrets: c.secrets, now: c.now}}, nil
}

// Publish confirms a complete validated batch atomically.
func (c *Context) Publish(batch []Capture) error {
	values, err := c.captureValues(batch)
	if err != nil {
		return err
	}
	if c.captures == nil {
		c.captures = make(map[string]variable)
	}
	for i, capture := range batch {
		c.captures[capture.Name] = values[i]
	}
	return nil
}
