// Package assertion evaluates assertions independently of execution and output.
package assertion

// Status reports whether the fully received response has the expected status.
func Status(expected, received int) bool { return expected == received }
