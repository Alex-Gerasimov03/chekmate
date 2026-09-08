// Тексты отчётов: расходы, инфляция, история цены.
package bot

import (
	"fmt"
	"strings"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
)

func formatReport(r analytics.Report) string {
	if r.Total == 0 && r.PreviousTotal == 0 {
		return "За этот период трат пока нет. Пришлите фото QR-кода с чека."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<b>%s — %s</b>\nИтого: <b>%s</b>\n",
		formatDate(r.Current.From), formatDate(r.Current.To), r.Total)

	if change, ok := r.Change(); ok {
		fmt.Fprintf(&b, "Прошлый период: %s (%s)\n", r.PreviousTotal, percent(change))
	}
	b.WriteString("\n")

	for _, c := range r.Categories {
		if c.Current == 0 {
			continue
		}
		fmt.Fprintf(&b, "%-22s %10s", escape(c.Title), c.Current.String())
		if change, ok := c.Change(); ok {
			fmt.Fprintf(&b, "  %s", percent(change))
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "\nЕда: %.0f%%  ·  Чеков: %d", r.FoodShare()*100, r.Receipts)
	if r.Receipts > 0 {
		fmt.Fprintf(&b, "  ·  Средний: %s", r.AverageReceipt())
	}
	return b.String()
}

func formatInflation(idx analytics.Index, titles map[int64]string, months int) string {
	if idx.Matched == 0 {
		return "Данных пока мало. Индекс считается по товарам, купленным " +
			"хотя бы трижды в каждом из сравниваемых периодов — " +
			"обычно на это уходит около двух месяцев покупок."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<b>Ваша инфляция: %s</b>\n", percent(idx.Change))
	fmt.Fprintf(&b, "Считано по %d товарам вашей постоянной корзины.\n\n", idx.Matched)

	for _, c := range idx.ByCategory {
		title := titles[c.CategoryID]
		if title == "" {
			title = "Без категории"
		}
		fmt.Fprintf(&b, "%-22s %s\n", escape(title), percent(c.Change))
	}

	if movers := idx.TopMovers(3); len(movers) > 0 {
		b.WriteString("\nСильнее всего подорожало:\n")
		for _, m := range movers {
			if m.Change <= 0 {
				continue
			}
			fmt.Fprintf(&b, "· %s: %s → %s (%s)\n",
				escape(m.Title), m.Before, m.After, percent(m.Change))
		}
	}
	return b.String()
}

func formatPriceHistory(title string, points []analytics.PricePoint) string {
	if len(points) == 0 {
		return "По этому товару покупок пока нет."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<b>%s</b>\n\n", escape(title))

	for _, p := range points {
		fmt.Fprintf(&b, "%s  %s", p.At, p.UnitPrice)
		if p.Merchant != "" {
			fmt.Fprintf(&b, "  · %s", escape(p.Merchant))
		}
		b.WriteString("\n")
	}

	// Точки идут от свежих к старым, поэтому изменение считается наоборот.
	if len(points) > 1 {
		oldest, newest := points[len(points)-1].UnitPrice, points[0].UnitPrice
		if oldest > 0 {
			change := float64(newest-oldest) / float64(oldest)
			fmt.Fprintf(&b, "\nЗа всё время: %s", percent(change))
		}
	}
	return b.String()
}

// percent печатает изменение со знаком: без него непонятно, рост это или спад.

func percent(v float64) string {
	p := v * 100
	switch {
	case p > 0.05:
		return fmt.Sprintf("+%.1f%%", p)
	case p < -0.05:
		return fmt.Sprintf("%.1f%%", p)
	default:
		return "без изменений"
	}
}

func formatTop(idx analytics.Index) string {
	movers := idx.TopMovers(10)
	if len(movers) == 0 {
		return "Пока не по чему сравнивать цены. Нужны повторные покупки " +
			"одних и тех же товаров — обычно на это уходит пара месяцев."
	}

	var b strings.Builder
	b.WriteString("<b>Что подорожало сильнее всего</b>\n\n")

	shown := 0
	for _, m := range movers {
		if m.Change <= 0 {
			continue
		}
		fmt.Fprintf(&b, "· %s\n   %s → %s  (%s)\n",
			escape(m.Title), m.Before, m.After, percent(m.Change))
		shown++
	}
	if shown == 0 {
		return "Из ваших постоянных товаров пока ничего не подорожало."
	}
	return b.String()
}
