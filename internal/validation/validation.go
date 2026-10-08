// Package validation is the shared shape of input errors, so the API can
// report every invalid field of a request in one response.
package validation

import (
	"errors"
	"strings"
)

// FieldError describes one invalid field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error lists every invalid field of one input.
type Error struct {
	Fields []FieldError
}

// Error lists the invalid fields in one line, for logs.
func (e *Error) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Message
	}
	return "invalid input: " + strings.Join(parts, "; ")
}

// Fields returns the field errors in err, or nil when err is not a
// validation error.
func Fields(err error) []FieldError {
	var v *Error
	if errors.As(err, &v) {
		return v.Fields
	}
	return nil
}

// Collector gathers field errors while checking an input.
type Collector struct {
	fields []FieldError
}

// Add records an invalid field.
func (c *Collector) Add(field, message string) {
	c.fields = append(c.fields, FieldError{Field: field, Message: message})
}

// Err returns an *Error with all recorded fields, or nil if there are none.
func (c *Collector) Err() error {
	if len(c.fields) == 0 {
		return nil
	}
	return &Error{Fields: c.fields}
}
