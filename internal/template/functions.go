package template

import (
	"crypto/rand"
	"time"
)

// Built-in function names are exact, case-sensitive and reserved.
const (
	FunctionDate = "newDate"
	FunctionUUID = "newUuid"
)

// ReservedFunction reports whether name is an exact built-in function name.
// Descendant paths such as newDate.x remain ordinary variables.
func ReservedFunction(name string) bool {
	return name == FunctionDate || name == FunctionUUID
}

// ReservedDetail is the constant, value-free diagnostic for the reservation
// rule shared by CLI declarations, configuration and capture batches.
const ReservedDetail = "nome reservado para função embutida (newDate, newUuid)"

// functionValue produces a fresh value for every occurrence; nothing is cached.
func (c *Context) functionValue(name string) (string, bool, error) {
	switch name {
	case FunctionDate:
		return c.clock().Format(time.RFC3339), true, nil
	case FunctionUUID:
		value, err := newUUID()
		return value, true, err
	default:
		return "", false, nil
	}
}

// newUUID returns a canonical lowercase version 4 UUID from crypto/rand.
func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	const hexDigits = "0123456789abcdef"
	text := make([]byte, 0, 36)
	for i, b := range value {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			text = append(text, '-')
		}
		text = append(text, hexDigits[b>>4], hexDigits[b&0x0f])
	}
	return string(text), nil
}
