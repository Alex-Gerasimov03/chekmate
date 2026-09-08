// Package analytics считает отчёты и индекс цен по накопленным чекам.
package analytics

import "time"

// Period — полуинтервал [From, To).
type Period struct {
	From time.Time
	To   time.Time
}

func (p Period) Days() int { return int(p.To.Sub(p.From).Hours() / 24) }

// MonthToDate возвращает текущий месяц и сопоставимый отрезок предыдущего.
func MonthToDate(now time.Time, loc *time.Location) (current, previous Period) {
	now = now.In(loc)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	elapsed := now.Sub(start)

	prevStart := start.AddDate(0, -1, 0)
	return Period{From: start, To: now}, Period{From: prevStart, To: prevStart.Add(elapsed)}
}

// FullMonth возвращает указанный месяц целиком и предыдущий месяц целиком.
func FullMonth(year int, month time.Month, loc *time.Location) (current, previous Period) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)
	prevStart := start.AddDate(0, -1, 0)

	return Period{From: start, To: end}, Period{From: prevStart, To: start}
}
