package resolver

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
)

// mockWords is a small word bank for generated sample values. It stays in-repo
// so mock functions need no extra dependency.
var mockWords = []string{
	"atlas", "bay", "cedar", "delta", "ember", "flint", "grove", "harbor",
	"iris", "jade", "keel", "lumen", "maple", "north", "osprey", "pine",
	"quartz", "ridge", "summit", "tide", "umbra", "vale", "willow", "zenith",
}

func mockWord() string {
	return mockWords[rand.IntN(len(mockWords))]
}

// mockText returns a short title-cased phrase for string fields.
func mockText(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("mockText accepts no arguments")
	}
	n := 2 + rand.IntN(3)
	parts := make([]string, n)
	for i := range parts {
		parts[i] = mockWord()
	}
	s := strings.Join(parts, " ")
	return strings.ToUpper(s[:1]) + s[1:], nil
}

// mockNumber returns a random float64 for number fields.
func mockNumber(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("mockNumber accepts no arguments")
	}
	return float64(rand.IntN(100000)) / 100, nil
}

// mockInteger returns a random int64 for integer fields.
func mockInteger(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("mockInteger accepts no arguments")
	}
	return int64(rand.IntN(1000) + 1), nil
}

// mockBoolean returns a random bool for boolean fields.
func mockBoolean(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("mockBoolean accepts no arguments")
	}
	return rand.IntN(2) == 1, nil
}

// mockDate returns a random YYYY-MM-DD string for date fields.
func mockDate(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("mockDate accepts no arguments")
	}
	return randomDay().Format(time.DateOnly), nil
}

// mockDateTime returns a random RFC 3339 timestamp for date-time fields.
func mockDateTime(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("mockDateTime accepts no arguments")
	}
	return randomDay().UTC().Format(time.RFC3339), nil
}

// mockEmail returns a random example.com address for email fields.
func mockEmail(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("mockEmail accepts no arguments")
	}
	return fmt.Sprintf("%s.%s@example.com", mockWord(), mockWord()), nil
}

// mockURL returns a random https URL for uri fields.
func mockURL(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("mockURL accepts no arguments")
	}
	return fmt.Sprintf("https://example.com/%s", mockWord()), nil
}

// mockPhone returns a fictional NANP number for phone fields.
func mockPhone(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("mockPhone accepts no arguments")
	}
	return fmt.Sprintf("+1%03d55501%02d", 200+rand.IntN(800), rand.IntN(100)), nil
}

// mockChoice picks one of the given arguments, for enum/choices fields.
func mockChoice(values ...any) (any, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("mockChoice requires at least 1 argument")
	}
	return values[rand.IntN(len(values))], nil
}

func randomDay() time.Time {
	start := time.Date(2020, 1, 1, 9, 0, 0, 0, time.UTC)
	return start.AddDate(0, 0, rand.IntN(365*6)).Add(time.Duration(rand.IntN(24)) * time.Hour)
}
