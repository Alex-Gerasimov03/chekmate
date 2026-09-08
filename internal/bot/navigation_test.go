package bot

import (
	"strings"
	"testing"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

func TestFormatCategory(t *testing.T) {
	got := formatCategory("Молочное и яйца", "Этот месяц", []analytics.ProductSpend{
		{ProductID: 1, Title: "молоко простоквашино", Spend: domain.MustParseMoney("267.00"),
			Purchases: 3, LastPrice: domain.MustParseMoney("89.00")},
		{ProductID: 2, Title: "сыр российский", Spend: domain.MustParseMoney("400.00"), Purchases: 1},
	})

	for _, want := range []string{"Молочное и яйца", "Этот месяц", "молоко простоквашино",
		"267,00 ₽", "3 покупки", "Всего: <b>667,00 ₽</b>"} {
		if !strings.Contains(got, want) {
			t.Errorf("в тексте нет %q:\n%s", want, got)
		}
	}
}

func TestFormatCategoryEmpty(t *testing.T) {
	if got := formatCategory("Рыба", "Неделя", nil); !strings.Contains(got, "Покупок за этот период нет") {
		t.Errorf("ожидалось объяснение:\n%s", got)
	}
}

func TestFormatHistory(t *testing.T) {
	receipts := []analytics.ReceiptSummary{
		{ID: 2, At: time.Date(2026, time.September, 6, 13, 43, 0, 0, msk),
			Total: domain.MustParseMoney("917.94"), Merchant: "Магнит", Items: 6},
	}

	got := formatHistory("Этот месяц", receipts, 0, 1)
	for _, want := range []string{"История покупок", "6 сентября, 13:43", "917,94 ₽", "Магнит", "позиций: 6"} {
		if !strings.Contains(got, want) {
			t.Errorf("в истории нет %q:\n%s", want, got)
		}
	}
}

// Когда чеков больше страницы, пользователю показывают, где он находится.
func TestFormatHistoryShowsPaging(t *testing.T) {
	receipts := make([]analytics.ReceiptSummary, 8)
	for i := range receipts {
		receipts[i] = analytics.ReceiptSummary{ID: int64(i), At: time.Now(), Total: 100}
	}

	if got := formatHistory("Всё время", receipts, 8, 20); !strings.Contains(got, "9–16 из 20") {
		t.Errorf("нет указания страницы:\n%s", got)
	}
}

func TestFormatHistoryEmpty(t *testing.T) {
	if got := formatHistory("Неделя", nil, 0, 0); !strings.Contains(got, "покупок нет") {
		t.Errorf("ожидалось объяснение:\n%s", got)
	}
}

func TestFormatReceipt(t *testing.T) {
	head := analytics.ReceiptSummary{
		At:    time.Date(2026, time.September, 6, 13, 43, 0, 0, msk),
		Total: domain.MustParseMoney("917.94"), Merchant: "Магнит",
	}
	lines := []analytics.ReceiptLine{
		{RawName: "PURINA ONE Корм", Title: "purina one корм", Category: "Животные",
			Qty: analytics.Quantity{Milli: 1000, Unit: "pcs"}, Sum: domain.MustParseMoney("319.99")},
	}

	got := formatReceipt(head, lines)
	for _, want := range []string{"917,94 ₽", "6 сентября", "Магнит", "purina one корм", "Животные"} {
		if !strings.Contains(got, want) {
			t.Errorf("в карточке чека нет %q:\n%s", want, got)
		}
	}
}

// У чека без состава показывается хотя бы сумма.
func TestFormatReceiptWithoutItems(t *testing.T) {
	head := analytics.ReceiptSummary{At: time.Now(), Total: domain.MustParseMoney("100.00")}

	if got := formatReceipt(head, nil); !strings.Contains(got, "только сумма") {
		t.Errorf("ожидалось объяснение:\n%s", got)
	}
}

func TestShorten(t *testing.T) {
	if got := shorten("очень длинное название товара", 10); len([]rune(got)) != 10 {
		t.Errorf("длина %d, ожидалось 10: %q", len([]rune(got)), got)
	}
	if got := shorten("молоко", 10); got != "молоко" {
		t.Errorf("короткое название не должно меняться: %q", got)
	}
}

func TestNavArgAt(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		n        int
		fallback string
		want     string
	}{
		{"первый параметр", "42|m", 0, "", "42"},
		{"второй параметр", "42|m", 1, periodMonth, "m"},
		{"параметра нет", "42", 1, periodMonth, periodMonth},
		{"пустой параметр", "42|", 1, periodMonth, periodMonth},
		{"пробелы обрезаются", " 42 |m", 0, "", "42"},
		{"пустая строка", "", 0, "x", "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := navArgAt(tt.data, tt.n, tt.fallback); got != tt.want {
				t.Errorf("navArgAt(%q, %d) = %q, ожидалось %q", tt.data, tt.n, got, tt.want)
			}
		})
	}
}

// Клавиатура отчёта показывает только категории с тратами и отмечает
// выбранный период.
func TestReportKeyboard(t *testing.T) {
	cats := []analytics.CategorySpend{
		{CategoryID: 1, Title: "Мясо", Current: domain.MustParseMoney("500.00")},
		{CategoryID: 2, Title: "Рыба", Current: 0},
	}

	rows := reportKeyboard(cats, periodWeek).InlineKeyboard
	var labels []string
	for _, row := range rows {
		for _, btn := range row {
			labels = append(labels, btn.Text)
		}
	}
	all := strings.Join(labels, " | ")

	if !strings.Contains(all, "Мясо") {
		t.Errorf("категория с тратами должна быть кнопкой:\n%s", all)
	}
	if strings.Contains(all, "Рыба") {
		t.Errorf("пустая категория не должна показываться:\n%s", all)
	}
	if !strings.Contains(all, "· Неделя ·") {
		t.Errorf("выбранный период должен быть отмечен:\n%s", all)
	}
	if !strings.Contains(all, "История покупок") {
		t.Errorf("нет перехода в историю:\n%s", all)
	}
}

// Листание появляется только там, где есть куда листать.
func TestHistoryKeyboardPaging(t *testing.T) {
	receipts := []analytics.ReceiptSummary{{ID: 1, At: time.Now(), Total: 100}}

	first := buttonTexts(historyKeyboard(receipts, periodMonth, 0, 20))
	if strings.Contains(first, "←") && !strings.Contains(first, "К расходам") {
		t.Error("на первой странице не должно быть кнопки назад по страницам")
	}
	if !strings.Contains(first, "→") {
		t.Error("должна быть кнопка вперёд")
	}

	last := buttonTexts(historyKeyboard(receipts, periodMonth, 16, 20))
	if strings.Contains(last, "→") {
		t.Error("на последней странице не должно быть кнопки вперёд")
	}
}

func buttonTexts(m *tele.ReplyMarkup) string {
	var out []string
	for _, row := range m.InlineKeyboard {
		for _, btn := range row {
			out = append(out, btn.Text)
		}
	}
	return strings.Join(out, " | ")
}
