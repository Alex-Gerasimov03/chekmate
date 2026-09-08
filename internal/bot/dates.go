package bot

import (
	"fmt"
	"time"
)

// Go форматирует даты по образцу английского времени: строка "2 января"
// печатала бы январь круглый год, поэтому названия подставляются вручную.
var months = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

func monthName(m time.Month) string {
	if m < time.January || m > time.December {
		return ""
	}
	return months[m-1]
}

// formatDate — "6 сентября".
func formatDate(t time.Time) string {
	return fmt.Sprintf("%d %s", t.Day(), monthName(t.Month()))
}

// formatDateTime — "6 сентября, 13:43".
func formatDateTime(t time.Time) string {
	return fmt.Sprintf("%s, %02d:%02d", formatDate(t), t.Hour(), t.Minute())
}
