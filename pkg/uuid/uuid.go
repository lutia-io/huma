// Package uuid provides UUID v4 generation and canonical-string validation
// using only the standard library.
package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalid is returned by Parse when the input is not a canonical UUID.
var ErrInvalid = errors.New("invalid uuid")

// New returns a random UUID v4 in canonical 8-4-4-4-12 lowercase hex form.
func New() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating uuid: %w", err)
	}
	// Version 4 (random) and RFC 4122 variant.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return format(b), nil
}

// MustNew is like New but panics if the system entropy source fails.
func MustNew() string {
	id, err := New()
	if err != nil {
		panic(err)
	}
	return id
}

// Valid reports whether s is a canonical UUID string:
// 8-4-4-4-12 hexadecimal digits separated by hyphens (case-insensitive).
func Valid(s string) bool {
	hexGroups := []int{8, 4, 4, 4, 12}
	groups := strings.Split(s, "-")
	if len(groups) != len(hexGroups) {
		return false
	}
	for i, group := range groups {
		if len(group) != hexGroups[i] {
			return false
		}
		for _, ch := range group {
			switch {
			case ch >= '0' && ch <= '9':
			case ch >= 'a' && ch <= 'f':
			case ch >= 'A' && ch <= 'F':
			default:
				return false
			}
		}
	}
	return true
}

// Parse returns s when it is a valid canonical UUID, otherwise ErrInvalid.
func Parse(s string) (string, error) {
	if !Valid(s) {
		return "", ErrInvalid
	}
	return s, nil
}

func format(b [16]byte) string {
	var buf [36]byte
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf[:])
}
