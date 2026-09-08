package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
	"github.com/Alex-Gerasimov03/chekmate/internal/service"
)

var msk = time.FixedZone("MSK", 3*60*60)

func TestFormatAddedDuplicate(t *testing.T) {
	got := formatAdded(service.Added{
		Created: false,
		Receipt: domain.Receipt{
			Total: domain.MustParseMoney("1543.20"),
			At:    time.Date(2026, 9, 6, 12, 15, 0, 0, msk),
		},
	})

	if !strings.Contains(got, "уже добавлен") {
		t.Errorf("повторный чек должен сообщать об этом:\n%s", got)
	}
}

func TestFormatAddedShowsItems(t *testing.T) {
	got := formatAdded(service.Added{
		Created: true,
		Receipt: domain.Receipt{
			Total: domain.MustParseMoney("134.50"),
			At:    time.Date(2026, 9, 6, 12, 15, 0, 0, msk),
			Items: []domain.Item{{RawName: "МОЛОКО", Sum: domain.MustParseMoney("89.00")}},
		},
	})

	for _, want := range []string{"134,50 ₽", "МОЛОКО", "89,00 ₽"} {
		if !strings.Contains(got, want) {
			t.Errorf("в сообщении нет %q:\n%s", want, got)
		}
	}
}

// Названия товаров приходят из чеков и могут содержать угловые скобки.
func TestFormatEscapesHTML(t *testing.T) {
	got := formatAdded(service.Added{
		Created: true,
		Receipt: domain.Receipt{
			At:    time.Now(),
			Items: []domain.Item{{RawName: "<b>сыр</b>", Sum: 100}},
		},
	})

	if strings.Contains(got, "<b>сыр") {
		t.Errorf("разметка из чека должна экранироваться:\n%s", got)
	}
}

func TestFormatReportEmpty(t *testing.T) {
	current, previous := analytics.MonthToDate(time.Now(), msk)

	if got := formatReport(analytics.BuildReport(current, previous, nil, 0)); !strings.Contains(got, "трат пока нет") {
		t.Errorf("пустой отчёт должен объяснять, что делать:\n%s", got)
	}
}

func TestFormatReport(t *testing.T) {
	current, previous := analytics.MonthToDate(time.Date(2026, 9, 6, 12, 0, 0, 0, msk), msk)
	r := analytics.BuildReport(current, previous, []analytics.CategorySpend{
		{CategoryID: 1, Title: "Мясо", IsFood: true,
			Current: domain.MustParseMoney("1980.00"), Previous: domain.MustParseMoney("1400.00")},
	}, 3)

	got := formatReport(r)
	for _, want := range []string{"Мясо", "1 980,00 ₽", "+41.4%", "Чеков: 3"} {
		if !strings.Contains(got, want) {
			t.Errorf("в отчёте нет %q:\n%s", want, got)
		}
	}
}

// Пока данных мало, бот обязан объяснить, почему индекса нет.
func TestFormatInflationWithoutData(t *testing.T) {
	if got := formatInflation(analytics.Index{}, nil, 3); !strings.Contains(got, "Данных пока мало") {
		t.Errorf("ожидалось объяснение:\n%s", got)
	}
}

func TestFormatInflation(t *testing.T) {
	idx := analytics.Index{
		Change:     0.074,
		Matched:    47,
		ByCategory: []analytics.CategoryChange{{CategoryID: 1, Change: 0.142}},
		Movers: []analytics.Mover{{
			Title:  "масло сливочное",
			Before: domain.MustParseMoney("1240.00"),
			After:  domain.MustParseMoney("1490.00"),
			Change: 0.2,
		}},
	}

	got := formatInflation(idx, map[int64]string{1: "Молочное"}, 3)
	for _, want := range []string{"+7.4%", "47 товарам", "Молочное", "+14.2%", "масло сливочное"} {
		if !strings.Contains(got, want) {
			t.Errorf("в сводке нет %q:\n%s", want, got)
		}
	}
}

func TestFormatPriceHistory(t *testing.T) {
	got := formatPriceHistory("кофе", []analytics.PricePoint{
		{At: "01.09.2026", Merchant: "Лента", UnitPrice: domain.MustParseMoney("390.00")},
		{At: "01.08.2026", Merchant: "Пятёрочка", UnitPrice: domain.MustParseMoney("300.00")},
	})

	for _, want := range []string{"кофе", "390,00 ₽", "Лента", "+30.0%"} {
		if !strings.Contains(got, want) {
			t.Errorf("в истории нет %q:\n%s", want, got)
		}
	}
}

func TestPercent(t *testing.T) {
	for _, tt := range []struct {
		in   float64
		want string
	}{
		{0.074, "+7.4%"},
		{-0.034, "-3.4%"},
		{0, "без изменений"},
	} {
		if got := percent(tt.in); got != tt.want {
			t.Errorf("percent(%v) = %q, ожидалось %q", tt.in, got, tt.want)
		}
	}
}

// Нажатие кнопки не должно попадать в траты как текст.
func TestMenuButtonsAreRecognised(t *testing.T) {
	for _, text := range []string{"📊 Расходы", "📈 Инфляция", "🔺 Подорожало", "❓ Помощь"} {
		if !isMenuButton(text) {
			t.Errorf("%q не опознан как кнопка меню", text)
		}
	}
	if isMenuButton("молоко 89") {
		t.Error("обычная трата опознана как кнопка")
	}
}

func TestFormatTopEmpty(t *testing.T) {
	if got := formatTop(analytics.Index{}); !strings.Contains(got, "Пока не по чему сравнивать") {
		t.Errorf("ожидалось объяснение:\n%s", got)
	}
}

func TestFormatTop(t *testing.T) {
	idx := analytics.Index{Movers: []analytics.Mover{
		{Title: "масло сливочное", Before: domain.MustParseMoney("1240.00"),
			After: domain.MustParseMoney("1490.00"), Change: 0.2},
		{Title: "молоко", Before: domain.MustParseMoney("100.00"),
			After: domain.MustParseMoney("90.00"), Change: -0.1},
	}}

	got := formatTop(idx)
	if !strings.Contains(got, "масло сливочное") {
		t.Errorf("подорожавший товар пропал:\n%s", got)
	}
	if strings.Contains(got, "молоко") {
		t.Errorf("подешевевший товар не должен попадать в список:\n%s", got)
	}
}

func TestPlural(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "1 позицию"}, {2, "2 позиции"}, {5, "5 позиций"},
		{11, "11 позиций"}, {21, "21 позицию"}, {104, "104 позиции"},
	}

	for _, tt := range tests {
		if got := plural(tt.n, "позицию", "позиции", "позиций"); got != tt.want {
			t.Errorf("plural(%d) = %q, ожидалось %q", tt.n, got, tt.want)
		}
	}
}

// Длинный чек не разворачивается в простыню: подробности за кнопкой.
func TestFormatAddedCollapsesLongReceipt(t *testing.T) {
	items := make([]domain.Item, 7)
	for i := range items {
		items[i] = domain.Item{RawName: "ТОВАР", Sum: 10000}
	}

	got := formatAdded(service.Added{Created: true, Receipt: domain.Receipt{
		At: time.Now(), Total: 70000, Merchant: "Лента", Items: items,
	}})

	if !strings.Contains(got, "Позиций: 7") {
		t.Errorf("ожидалось число позиций:\n%s", got)
	}
	if strings.Count(got, "ТОВАР") > 0 {
		t.Errorf("длинный список не должен разворачиваться:\n%s", got)
	}
	if !strings.Contains(got, "Лента") {
		t.Errorf("магазин должен показываться:\n%s", got)
	}
}
