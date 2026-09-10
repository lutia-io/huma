// Package uuid wraps the standard library uuid package with the string-oriented
// helpers used across the service (canonical 8-4-4-4-12 form).
package uuid

import (
	"errors"
	stduuid "uuid"
)

// ErrInvalid is returned by Parse when the input is not a canonical UUID.
var ErrInvalid = errors.New("invalid uuid")

// New returns a random UUID v4 in canonical 8-4-4-4-12 lowercase hex form.
func New() (string, error) {
	return stduuid.New().String(), nil
}

// MustNew is like New but panics if generation fails.
func MustNew() string {
	return stduuid.New().String()
}

// Valid reports whether s is a canonical UUID string:
// 8-4-4-4-12 hexadecimal digits separated by hyphens (case-insensitive).
func Valid(s string) bool {
	if len(s) != 36 {
		return false
	}
	_, err := stduuid.Parse(s)
	return err == nil
}

// Parse returns s when it is a valid canonical UUID, otherwise ErrInvalid.
func Parse(s string) (string, error) {
	if !Valid(s) {
		return "", ErrInvalid
	}
	return s, nil
}
