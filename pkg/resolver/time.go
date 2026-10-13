package resolver

import (
	"fmt"
	"time"
)

const timeLayout = "15:04:05Z07:00"

type parsedInstant struct {
	t       time.Time
	hasDate bool
	hasTime bool
}

// nowDateTime returns the current UTC time in RFC 3339 for date-time fields.
func nowDateTime(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("nowDateTime accepts no arguments")
	}
	return nowFunc().UTC().Format(time.RFC3339), nil
}

// now is an alias of nowDateTime.
func now(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("now accepts no arguments")
	}
	return nowDateTime()
}

// nowDate returns the current UTC date as YYYY-MM-DD for date fields.
func nowDate(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("nowDate accepts no arguments")
	}
	return nowFunc().UTC().Format(time.DateOnly), nil
}

// nowTime returns the current UTC clock time as an RFC 3339 full-time for time fields.
func nowTime(values ...any) (any, error) {
	if len(values) != 0 {
		return nil, fmt.Errorf("nowTime accepts no arguments")
	}
	return nowFunc().UTC().Format(timeLayout), nil
}

// dateOf extracts the YYYY-MM-DD date from a date-time or date string.
func dateOf(values ...any) (any, error) {
	if len(values) != 1 {
		return nil, fmt.Errorf("dateOf requires 1 argument")
	}
	p, err := parseInstant(values[0])
	if err != nil {
		return nil, fmt.Errorf("dateOf: %w", err)
	}
	if !p.hasDate {
		return nil, fmt.Errorf("dateOf: value has no date")
	}
	return p.t.UTC().Format(time.DateOnly), nil
}

// timeOf extracts the RFC 3339 full-time from a date-time or time string.
func timeOf(values ...any) (any, error) {
	if len(values) != 1 {
		return nil, fmt.Errorf("timeOf requires 1 argument")
	}
	p, err := parseInstant(values[0])
	if err != nil {
		return nil, fmt.Errorf("timeOf: %w", err)
	}
	if !p.hasTime {
		return nil, fmt.Errorf("timeOf: value has no time")
	}
	return p.t.UTC().Format(timeLayout), nil
}

func parseInstant(v any) (parsedInstant, error) {
	s, ok := v.(string)
	if !ok || s == "" {
		return parsedInstant{}, fmt.Errorf("not a date, time, or date-time string")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return parsedInstant{t: t, hasDate: true, hasTime: true}, nil
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return parsedInstant{t: t, hasDate: true, hasTime: false}, nil
	}
	if t, err := time.Parse(timeLayout, s); err == nil {
		return parsedInstant{t: t, hasDate: false, hasTime: true}, nil
	}
	return parsedInstant{}, fmt.Errorf("not a date, time, or date-time string")
}
