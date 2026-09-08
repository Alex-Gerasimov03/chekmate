// Обработчики отчётов: расходы, инфляция, цены и правила категорий.
package bot

import (
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
)

func (b *Bot) onReport(c tele.Context) error {
	ctx, cancel := b.ctx()
	defer cancel()

	budgetID, err := b.budget(ctx, c)
	if err != nil {
		return b.fail(c, "отчёт", err)
	}

	report, cats, err := b.buildReport(ctx, budgetID, periodMonth)
	if err != nil {
		return b.fail(c, "отчёт", err)
	}
	return c.Send(formatReport(report), reportKeyboard(cats, periodMonth))
}

func (b *Bot) onInflation(c tele.Context) error {
	ctx, cancel := b.ctx()
	defer cancel()

	budgetID, err := b.budget(ctx, c)
	if err != nil {
		return b.fail(c, "индекс", err)
	}

	months := defaultInflationMonths
	if c.Message() != nil {
		if n, err := parseMonths(c.Message().Payload); err == nil {
			months = n
		}
	}

	base, current := inflationPeriods(time.Now().In(b.loc), months)

	baseObs, err := b.stats.PriceObservations(ctx, budgetID, base)
	if err != nil {
		return b.fail(c, "индекс", err)
	}
	currentObs, err := b.stats.PriceObservations(ctx, budgetID, current)
	if err != nil {
		return b.fail(c, "индекс", err)
	}
	titles, err := b.stats.CategoryTitles(ctx)
	if err != nil {
		return b.fail(c, "индекс", err)
	}

	idx := analytics.Inflate(baseObs, currentObs, analytics.MinPurchases)
	return c.Send(formatInflation(idx, titles, months))
}

func (b *Bot) onPrice(c tele.Context) error {
	query := strings.TrimSpace(c.Message().Payload)
	if query == "" {
		return c.Send("Укажите товар: <code>/price молоко</code>")
	}

	ctx, cancel := b.ctx()
	defer cancel()

	budgetID, err := b.budget(ctx, c)
	if err != nil {
		return b.fail(c, "цены", err)
	}

	found, err := b.stats.SearchProducts(ctx, budgetID, query, 1)
	if err != nil {
		return b.fail(c, "цены", err)
	}
	if len(found) == 0 {
		return c.Send("Такого товара в ваших чеках нет.")
	}

	points, err := b.stats.PriceHistory(ctx, budgetID, found[0].ProductID, 10)
	if err != nil {
		return b.fail(c, "цены", err)
	}
	return c.Send(formatPriceHistory(found[0].Title, points))
}

const defaultInflationMonths = 3

// inflationPeriods возвращает базовый и текущий отрезки для сравнения цен.

func inflationPeriods(now time.Time, months int) (base, current analytics.Period) {
	return analytics.Period{From: now.AddDate(0, -months-1, 0), To: now.AddDate(0, -months, 0)},
		analytics.Period{From: now.AddDate(0, -1, 0), To: now}
}

func (b *Bot) onRule(c tele.Context) error {
	parts := strings.Fields(strings.TrimSpace(c.Message().Payload))
	if len(parts) < 2 {
		return b.sendCategories(c)
	}

	code := parts[len(parts)-1]
	word := strings.Join(parts[:len(parts)-1], " ")

	ctx, cancel := b.ctx()
	defer cancel()

	categoryID, err := b.cats.ByCode(ctx, code)
	if err != nil {
		return b.sendCategories(c)
	}

	budgetID, err := b.budget(ctx, c)
	if err != nil {
		return b.fail(c, "правило", err)
	}

	changed, err := b.svc.AddRule(ctx, budgetID, categoryID, word)
	if err != nil {
		return b.fail(c, "правило", err)
	}
	return c.Send(formatRuleAdded(word, code, changed))
}

func (b *Bot) sendCategories(c tele.Context) error {
	ctx, cancel := b.ctx()
	defer cancel()

	cats, err := b.cats.All(ctx)
	if err != nil {
		return b.fail(c, "справочник", err)
	}
	return c.Send(formatCategories(cats))
}

// head показывает первые байты файла: по ним видно, что именно прислали,
// если декодер не справился.

func (b *Bot) onTop(c tele.Context) error {
	ctx, cancel := b.ctx()
	defer cancel()

	budgetID, err := b.budget(ctx, c)
	if err != nil {
		return b.fail(c, "подорожание", err)
	}

	base, current := inflationPeriods(time.Now().In(b.loc), defaultInflationMonths)

	baseObs, err := b.stats.PriceObservations(ctx, budgetID, base)
	if err != nil {
		return b.fail(c, "подорожание", err)
	}
	currentObs, err := b.stats.PriceObservations(ctx, budgetID, current)
	if err != nil {
		return b.fail(c, "подорожание", err)
	}

	idx := analytics.Inflate(baseObs, currentObs, analytics.MinPurchases)
	return c.Send(formatTop(idx))
}
