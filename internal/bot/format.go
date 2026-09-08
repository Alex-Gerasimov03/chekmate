// Сообщения собираются здесь отдельно от обработчиков: так их можно
// проверить целиком, не поднимая Telegram.
package bot

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
	"github.com/Alex-Gerasimov03/chekmate/internal/service"
)

// Сообщения собираются здесь отдельно от обработчиков: так их можно
// проверить целиком, не поднимая Telegram.

func formatAdded(a service.Added) string {
	var b strings.Builder

	if !a.Created {
		fmt.Fprintf(&b, "Этот чек уже добавлен: %s от %s\n",
			a.Receipt.Total, formatDateTime(a.Receipt.At))
		return b.String()
	}

	fmt.Fprintf(&b, "Добавлено: <b>%s</b>\n%s", a.Receipt.Total, formatDateTime(a.Receipt.At))
	if a.Receipt.Merchant != "" {
		fmt.Fprintf(&b, "\n%s", escape(a.Receipt.Merchant))
	}
	b.WriteString("\n")

	// Длинный чек не разворачивается в простыню: подробности за кнопкой.
	switch n := len(a.Receipt.Items); {
	case n == 0:
		b.WriteString("\nСостав чека получить не удалось — учтена только сумма.\n")
	case n <= 4:
		b.WriteString("\n")
		for _, it := range a.Receipt.Items {
			fmt.Fprintf(&b, "· %s — %s\n", escape(shorten(it.RawName, 32)), it.Sum)
		}
	default:
		fmt.Fprintf(&b, "\nПозиций: %d\n", n)
	}

	if len(a.Questions) > 0 {
		fmt.Fprintf(&b, "\nНужно уточнить %s.\n",
			plural(len(a.Questions), "позицию", "позиции", "позиций"))
	}
	return b.String()
}

// plural выбирает форму слова: "1 позицию", "2 позиции", "5 позиций".
func plural(n int, one, few, many string) string {
	word := many
	switch {
	case n%10 == 1 && n%100 != 11:
		word = one
	case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
		word = few
	}
	return fmt.Sprintf("%d %s", n, word)
}

func formatQuestion(q service.Question) string {
	return fmt.Sprintf("Позиция <b>%s</b>\nЭто тот же товар, что «%s»?",
		escape(q.RawName), escape(q.Suggestion.Title))
}

// escape обезвреживает текст из чеков: названия товаров попадают
// в разметку сообщения как есть.
func escape(s string) string { return html.EscapeString(s) }

// parseMonths разбирает аргумент /inflation: за сколько месяцев сравнивать.

func parseMonths(payload string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(payload))
	if err != nil {
		return 0, err
	}
	if n < 1 || n > 24 {
		return 0, fmt.Errorf("период вне разумных границ: %d", n)
	}
	return n, nil
}

func formatRuleAdded(word, code string, changed int) string {
	s := fmt.Sprintf("Запомнил: <b>%s</b> — это %s.", escape(word), escape(code))
	if changed > 0 {
		s += fmt.Sprintf("\nПересчитал категории у %d товаров.", changed)
	}
	return s
}

func formatCategories(cats []analytics.Category) string {
	var b strings.Builder
	b.WriteString("Правило задаётся так: <code>/rule сгущенка sweets</code>\n\nКатегории:\n")

	for _, c := range cats {
		mark := " "
		if c.IsFood {
			mark = "·"
		}
		fmt.Fprintf(&b, "%s <code>%s</code> — %s\n", mark, c.Code, escape(c.Title))
	}
	return b.String()
}
