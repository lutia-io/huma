package workflow

import (
	"testing"
	"time"
)

func TestScheduleDueAt_dailyNinePacific(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}

	due, err := ScheduleDueAt("0 9 * * *", "America/Los_Angeles", time.Date(2026, 9, 8, 9, 0, 30, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	if !due {
		t.Fatal("expected 09:00 Pacific to be due")
	}

	due, err = ScheduleDueAt("0 9 * * *", "America/Los_Angeles", time.Date(2026, 9, 8, 9, 1, 0, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	if due {
		t.Fatal("expected 09:01 Pacific not to be due")
	}
}

func TestScheduleDueAt_utcVsPacific(t *testing.T) {
	// 16:00 UTC is 09:00 PDT on 8 Sep 2026.
	at := time.Date(2026, 9, 8, 16, 0, 0, 0, time.UTC)
	due, err := ScheduleDueAt("0 9 * * *", "America/Los_Angeles", at)
	if err != nil {
		t.Fatal(err)
	}
	if !due {
		t.Fatal("expected UTC instant that is 09:00 Pacific to be due")
	}
}

func TestParseSchedule_invalid(t *testing.T) {
	if _, _, err := ParseSchedule("not-cron", "UTC"); err == nil {
		t.Fatal("expected invalid cron")
	}
	if _, _, err := ParseSchedule("0 9 * * *", "Not/AZone"); err == nil {
		t.Fatal("expected invalid timezone")
	}
}
