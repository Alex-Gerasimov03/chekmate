// Тексты детализации: товары категории, история покупок, карточка чека.
package bot

import (
	"fmt"
	"strings"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

func formatCategory(category, period string, items []analytics.ProductSpend) string {
	if len(items) == 0 {
		return fmt.Sprintf("<b>%s</b> · %s\n\nПокупок за этот период нет.",
			escape(category), escape(period))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<b>%s</b> · %s\n\n", escape(category), escape(period))

	var total domain.Money
	for _, it := range items {
		total += it.Spend
		fmt.Fprintf(&b, "· %s\n   %s", escape(it.Title), it.Spend)
		if it.Purchases > 1 {
			fmt.Fprintf(&b, "  ·  %d покупки по %s", it.Purchases, it.LastPrice)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\nВсего: <b>%s</b>", total)
	return b.String()
}

func formatHistory(period string, receipts []analytics.ReceiptSummary, offset, total int) string {
	if total == 0 {
		return fmt.Sprintf("<b>История покупок</b> · %s\n\nЗа этот период покупок нет.", escape(period))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<b>История покупок</b> · %s\nВсего чеков: %d\n\n", escape(period), total)

	var sum domain.Money
	for _, r := range receipts {
		sum += r.Total
		fmt.Fprintf(&b, "· %s — <b>%s</b>", formatDateTime(r.At), r.Total)
		if r.Merchant != "" {
			fmt.Fprintf(&b, "\n   %s", escape(r.Merchant))
		}
		if r.Items > 0 {
			fmt.Fprintf(&b, "  ·  позиций: %d", r.Items)
		}
		b.WriteString("\n")
	}

	if total > len(receipts) {
		fmt.Fprintf(&b, "\nПоказаны %d–%d из %d", offset+1, offset+len(receipts), total)
	}
	return b.String()
}

func formatReceipt(head analytics.ReceiptSummary, lines []analytics.ReceiptLine) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<b>Чек на %s</b>\n%s", head.Total, formatDateTime(head.At))
	if head.Merchant != "" {
		fmt.Fprintf(&b, "\n%s", escape(head.Merchant))
	}
	b.WriteString("\n\n")

	if len(lines) == 0 {
		b.WriteString("Состав чека получить не удалось — учтена только сумма.")
		return b.String()
	}

	for _, l := range lines {
		name := l.Title
		if name == "" {
			name = l.RawName
		}
		fmt.Fprintf(&b, "· %s\n   %s", escape(name), l.Sum)
		if q := formatQty(l.Qty); q != "" {
			fmt.Fprintf(&b, "  ·  %s", q)
		}
		if l.Category != "" {
			fmt.Fprintf(&b, "  ·  %s", escape(l.Category))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// formatQty печатает количество позиции, пропуская привычную единицу товара.

func formatQty(q analytics.Quantity) string {
	if q.Milli == 0 {
		return ""
	}
	return domain.NewQuantity(q.Milli, domain.Unit(q.Unit)).String()
}
