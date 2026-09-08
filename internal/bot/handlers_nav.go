package bot

import (
	"context"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
)

// Навигация построена на одном сообщении, которое переписывается на месте:

func (b *Bot) onNavReport(c tele.Context) error {
	period := navArg(c, 0, periodMonth)

	ctx, cancel := b.ctx()
	defer cancel()

	budgetID, err := b.budget(ctx, c)
	if err != nil {
		return b.fail(c, "отчёт", err)
	}

	report, cats, err := b.buildReport(ctx, budgetID, period)
	if err != nil {
		return b.fail(c, "отчёт", err)
	}
	return b.edit(c, formatReport(report), reportKeyboard(cats, period))
}

func (b *Bot) onNavCategory(c tele.Context) error {
	categoryID, err := strconv.ParseInt(navArg(c, 0, ""), 10, 64)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Категория не найдена"})
	}
	period := navArg(c, 1, periodMonth)

	ctx, cancel := b.ctx()
	defer cancel()

	budgetID, err := b.budget(ctx, c)
	if err != nil {
		return b.fail(c, "категория", err)
	}

	p, title := resolvePeriod(period, time.Now(), b.loc)
	items, err := b.stats.ProductsInCategory(ctx, budgetID, categoryID, p, pageSize)
	if err != nil {
		return b.fail(c, "категория", err)
	}

	titles, err := b.stats.CategoryTitles(ctx)
	if err != nil {
		return b.fail(c, "категория", err)
	}
	return b.edit(c, formatCategory(titles[categoryID], title, items), categoryKeyboard(items, period))
}

func (b *Bot) onNavProduct(c tele.Context) error {
	productID, err := strconv.ParseInt(navArg(c, 0, ""), 10, 64)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Товар не найден"})
	}
	period := navArg(c, 1, periodMonth)

	ctx, cancel := b.ctx()
	defer cancel()

	budgetID, err := b.budget(ctx, c)
	if err != nil {
		return b.fail(c, "цены", err)
	}

	points, err := b.stats.PriceHistory(ctx, budgetID, productID, 12)
	if err != nil {
		return b.fail(c, "цены", err)
	}

	title, err := b.stats.ProductTitle(ctx, productID)
	if err != nil {
		return b.fail(c, "цены", err)
	}
	return b.edit(c, formatPriceHistory(title, points), productKeyboard(period))
}

func (b *Bot) onNavHistory(c tele.Context) error {
	period := navArg(c, 0, periodMonth)
	offset, _ := strconv.Atoi(navArg(c, 1, "0"))

	ctx, cancel := b.ctx()
	defer cancel()

	budgetID, err := b.budget(ctx, c)
	if err != nil {
		return b.fail(c, "история", err)
	}

	p, title := resolvePeriod(period, time.Now(), b.loc)
	receipts, err := b.stats.ReceiptsInPeriod(ctx, budgetID, p, pageSize, offset)
	if err != nil {
		return b.fail(c, "история", err)
	}
	total, err := b.stats.CountReceipts(ctx, budgetID, p)
	if err != nil {
		return b.fail(c, "история", err)
	}
	return b.edit(c, formatHistory(title, receipts, offset, total),
		historyKeyboard(receipts, period, offset, total))
}

func (b *Bot) onNavReceipt(c tele.Context) error {
	receiptID, err := strconv.ParseInt(navArg(c, 0, ""), 10, 64)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Чек не найден"})
	}
	period := navArg(c, 1, periodMonth)

	ctx, cancel := b.ctx()
	defer cancel()

	budgetID, err := b.budget(ctx, c)
	if err != nil {
		return b.fail(c, "чек", err)
	}

	head, lines, err := b.stats.ReceiptLines(ctx, budgetID, receiptID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Чек не найден"})
	}
	return b.edit(c, formatReceipt(head, lines), receiptKeyboard(period))
}

// navArg достаёт n-й параметр нажатой кнопки.
func navArg(c tele.Context, n int, fallback string) string {
	return navArgAt(c.Data(), n, fallback)
}

// navArgAt разбирает параметры кнопки. Параметра может не быть: кнопку могла
// нарисовать прошлая версия бота.
func navArgAt(data string, n int, fallback string) string {
	parts := strings.Split(data, "|")
	if n >= len(parts) {
		return fallback
	}
	if v := strings.TrimSpace(parts[n]); v != "" {
		return v
	}
	return fallback
}

// buildReport собирает отчёт за произвольный период.
func (b *Bot) buildReport(ctx context.Context, budgetID int64, period string) (analytics.Report, []analytics.CategorySpend, error) {
	now := time.Now()
	current, _ := resolvePeriod(period, now, b.loc)
	previous := previousPeriod(period, current, now, b.loc)

	rows, err := b.stats.CategorySpends(ctx, budgetID, current, previous)
	if err != nil {
		return analytics.Report{}, nil, err
	}
	receipts, err := b.stats.ReceiptCount(ctx, budgetID, current)
	if err != nil {
		return analytics.Report{}, nil, err
	}

	report := analytics.BuildReport(current, previous, rows, receipts)
	return report, report.Categories, nil
}
