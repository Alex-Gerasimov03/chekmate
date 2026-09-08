package bot

import (
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
)

// Период кодируется одной буквой: она уезжает в данные кнопки, где на всё
// про всё 64 байта.
const (
	periodMonth = "m" // текущий месяц
	periodPrev  = "p" // прошлый месяц целиком
	periodWeek  = "w" // последние 7 дней
	periodAll   = "a" // всё время
)

// periodOrder задаёт, в каком порядке рисуются переключатели.
var periodOrder = []string{periodWeek, periodMonth, periodPrev, periodAll}

var periodTitles = map[string]string{
	periodWeek:  "Неделя",
	periodMonth: "Этот месяц",
	periodPrev:  "Прошлый месяц",
	periodAll:   "Всё время",
}

// resolvePeriod разворачивает код в отрезок времени и его название.
func resolvePeriod(code string, now time.Time, loc *time.Location) (analytics.Period, string) {
	now = now.In(loc)

	switch code {
	case periodWeek:
		return analytics.Period{From: now.AddDate(0, 0, -7), To: now}, periodTitles[periodWeek]

	case periodPrev:
		_, prev := analytics.MonthToDate(now, loc)
		start := time.Date(prev.From.Year(), prev.From.Month(), 1, 0, 0, 0, 0, loc)
		return analytics.Period{From: start, To: start.AddDate(0, 1, 0)}, periodTitles[periodPrev]

	case periodAll:
		return analytics.Period{From: time.Date(2000, 1, 1, 0, 0, 0, 0, loc), To: now}, periodTitles[periodAll]

	default:
		current, _ := analytics.MonthToDate(now, loc)
		return current, periodTitles[periodMonth]
	}
}

// previousPeriod — сопоставимый прошлый отрезок для сравнения расходов.
func previousPeriod(code string, p analytics.Period, now time.Time, loc *time.Location) analytics.Period {
	switch code {
	case periodWeek:
		return analytics.Period{From: p.From.AddDate(0, 0, -7), To: p.From}

	case periodPrev:
		return analytics.Period{From: p.From.AddDate(0, -1, 0), To: p.From}

	case periodAll:
		// Сравнивать «всё время» не с чем.
		return analytics.Period{From: p.From, To: p.From}

	default:
		_, previous := analytics.MonthToDate(now.In(loc), loc)
		return previous
	}
}
