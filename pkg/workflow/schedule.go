package workflow

import (
	"fmt"
	"time"
	_ "time/tzdata"

	"github.com/robfig/cron/v3"
)

// ParseSchedule validates a 5-field cron expression and IANA timezone.
func ParseSchedule(cronExpr, timezone string) (cron.Schedule, *time.Location, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, nil, fmt.Errorf("unknown timezone %q", timezone)
	}
	sched, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cron expression")
	}
	return sched, loc, nil
}

// ScheduleDueAt reports whether cronExpr fires at the given instant in timezone.
// at is truncated to the minute; seconds and below are ignored.
func ScheduleDueAt(cronExpr, timezone string, at time.Time) (bool, error) {
	sched, loc, err := ParseSchedule(cronExpr, timezone)
	if err != nil {
		return false, err
	}
	now := at.In(loc).Truncate(time.Minute)
	next := sched.Next(now.Add(-time.Second))
	return next.Equal(now), nil
}
