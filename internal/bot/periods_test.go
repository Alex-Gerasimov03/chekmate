package bot

import (
	"testing"
	"time"
)

func TestResolvePeriod(t *testing.T) {
	now := time.Date(2026, time.September, 8, 15, 0, 0, 0, msk)

	tests := []struct {
		code      string
		wantTitle string
		wantFrom  time.Time
	}{
		{periodWeek, "Неделя", time.Date(2026, time.September, 1, 15, 0, 0, 0, msk)},
		{periodMonth, "Этот месяц", time.Date(2026, time.September, 1, 0, 0, 0, 0, msk)},
		{periodPrev, "Прошлый месяц", time.Date(2026, time.August, 1, 0, 0, 0, 0, msk)},
	}

	for _, tt := range tests {
		t.Run(tt.wantTitle, func(t *testing.T) {
			p, title := resolvePeriod(tt.code, now, msk)
			if title != tt.wantTitle {
				t.Errorf("название %q, ожидалось %q", title, tt.wantTitle)
			}
			if !p.From.Equal(tt.wantFrom) {
				t.Errorf("начало %s, ожидалось %s", p.From, tt.wantFrom)
			}
		})
	}
}

// Прошлый месяц берётся целиком, а не до текущего числа.
func TestResolvePeriodPrevMonthIsWhole(t *testing.T) {
	now := time.Date(2026, time.September, 8, 15, 0, 0, 0, msk)
	p, _ := resolvePeriod(periodPrev, now, msk)

	if p.From.Month() != time.August || p.From.Day() != 1 {
		t.Errorf("начало %s, ожидалось 1 августа", p.From)
	}
	if p.To.Month() != time.September || p.To.Day() != 1 {
		t.Errorf("конец %s, ожидалось 1 сентября", p.To)
	}
}

func TestResolvePeriodUnknownCodeFallsBackToMonth(t *testing.T) {
	now := time.Date(2026, time.September, 8, 15, 0, 0, 0, msk)

	if _, title := resolvePeriod("нет такого", now, msk); title != periodTitles[periodMonth] {
		t.Errorf("название %q, ожидался текущий месяц", title)
	}
}

// Сравнение недели с предыдущей неделей, месяца — с прошлым месяцем.
func TestPreviousPeriodMatchesLength(t *testing.T) {
	now := time.Date(2026, time.September, 8, 15, 0, 0, 0, msk)

	for _, code := range []string{periodWeek, periodMonth, periodPrev} {
		p, _ := resolvePeriod(code, now, msk)
		prev := previousPeriod(code, p, now, msk)

		if d := p.To.Sub(p.From) - prev.To.Sub(prev.From); d > time.Hour || d < -time.Hour {
			t.Errorf("%s: отрезки разной длины (%v против %v)", code, p.To.Sub(p.From), prev.To.Sub(prev.From))
		}
	}
}
