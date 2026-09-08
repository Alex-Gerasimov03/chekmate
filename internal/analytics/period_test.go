package analytics

import (
	"testing"
	"time"
)

var msk = time.FixedZone("MSK", 3*60*60)

// Незавершённый месяц сравнивается с равным отрезком предыдущего, иначе
// в начале месяца отчёт всегда показывал бы обвал расходов.
func TestMonthToDateComparesEqualStretches(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, msk)
	current, previous := MonthToDate(now, msk)

	if want := time.Date(2026, 9, 1, 0, 0, 0, 0, msk); !current.From.Equal(want) {
		t.Errorf("начало текущего периода %s, ожидалось %s", current.From, want)
	}
	if !current.To.Equal(now) {
		t.Errorf("конец текущего периода %s, ожидалось %s", current.To, now)
	}
	if want := time.Date(2026, 8, 1, 0, 0, 0, 0, msk); !previous.From.Equal(want) {
		t.Errorf("начало прошлого периода %s, ожидалось %s", previous.From, want)
	}
	if a, b := current.To.Sub(current.From), previous.To.Sub(previous.From); a != b {
		t.Errorf("отрезки разной длины: %v и %v", a, b)
	}
}

// Январь сравнивается с декабрём прошлого года.
func TestMonthToDateCrossesYear(t *testing.T) {
	_, previous := MonthToDate(time.Date(2026, 1, 5, 0, 0, 0, 0, msk), msk)

	if previous.From.Year() != 2025 || previous.From.Month() != time.December {
		t.Errorf("предыдущий период %s, ожидался декабрь 2025", previous.From)
	}
}

func TestFullMonth(t *testing.T) {
	current, previous := FullMonth(2026, time.August, msk)

	if current.Days() != 31 {
		t.Errorf("в августе %d дней", current.Days())
	}
	if previous.Days() != 31 {
		t.Errorf("в июле %d дней", previous.Days())
	}
	if !previous.To.Equal(current.From) {
		t.Error("периоды должны стыковаться без разрыва")
	}
}
