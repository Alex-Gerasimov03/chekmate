package bot

import (
	"testing"
	"time"
)

// Стандартный Format("2 января") печатал бы январь в любом месяце — ради
// этого свойства названия и подставляются вручную.
func TestFormatDateUsesRealMonth(t *testing.T) {
	tests := []struct {
		in   time.Time
		want string
	}{
		{time.Date(2026, time.September, 6, 12, 15, 0, 0, msk), "6 сентября"},
		{time.Date(2026, time.January, 1, 0, 0, 0, 0, msk), "1 января"},
		{time.Date(2026, time.December, 31, 23, 59, 0, 0, msk), "31 декабря"},
		{time.Date(2026, time.May, 9, 10, 0, 0, 0, msk), "9 мая"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatDate(tt.in); got != tt.want {
				t.Errorf("formatDate = %q, ожидалось %q", got, tt.want)
			}
		})
	}
}

func TestFormatDateTime(t *testing.T) {
	got := formatDateTime(time.Date(2026, time.September, 6, 13, 43, 0, 0, msk))
	if want := "6 сентября, 13:43"; got != want {
		t.Errorf("formatDateTime = %q, ожидалось %q", got, want)
	}
}
