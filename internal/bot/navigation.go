package bot

import (
	"strconv"

	tele "gopkg.in/telebot.v4"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
)

// Уникальные метки кнопок навигации. Телеграм отдаёт их обратно вместе
// с данными, и по ним обработчик понимает, куда пользователь провалился.
const (
	navReport   = "rep"  // отчёт за период
	navCategory = "cat"  // товары внутри категории
	navProduct  = "prod" // история цены товара
	navHistory  = "hist" // список чеков
	navReceipt  = "rcpt" // позиции одного чека
)

// pageSize ограничивает список: в сообщение помещается немного кнопок,
// а листать длинные полотна в телефоне неудобно.
const pageSize = 8

// reportKeyboard — категории, в которые можно провалиться, и переключатели
// периода под ними.
func reportKeyboard(cats []analytics.CategorySpend, period string) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	shown := 0
	for _, c := range cats {
		if c.Current == 0 || shown >= pageSize {
			continue
		}
		rows = append(rows, m.Row(m.Data(
			c.Title+" · "+c.Current.String(), navCategory,
			strconv.FormatInt(c.CategoryID, 10), period,
		)))
		shown++
	}

	rows = append(rows, periodRow(m, navReport, period))
	rows = append(rows, m.Row(m.Data("🧾 История покупок", navHistory, period, "0")))

	m.Inline(rows...)
	return m
}

// periodRow рисует переключатели периода, отмечая текущий.
func periodRow(m *tele.ReplyMarkup, unique, active string) tele.Row {
	var btns []tele.Btn
	for _, code := range periodOrder {
		title := periodTitles[code]
		if code == active {
			title = "· " + title + " ·"
		}
		btns = append(btns, m.Data(title, unique, code))
	}
	return m.Row(btns...)
}

// categoryKeyboard — товары категории, каждый ведёт в историю своей цены.
func categoryKeyboard(items []analytics.ProductSpend, period string) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	for i, it := range items {
		if i >= pageSize {
			break
		}
		rows = append(rows, m.Row(m.Data(
			shorten(it.Title, 28)+" · "+it.Spend.String(), navProduct,
			strconv.FormatInt(it.ProductID, 10), period,
		)))
	}
	rows = append(rows, m.Row(m.Data("← К категориям", navReport, period)))

	m.Inline(rows...)
	return m
}

// historyKeyboard — список чеков с листанием.
func historyKeyboard(receipts []analytics.ReceiptSummary, period string, offset, total int) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	var rows []tele.Row

	for _, r := range receipts {
		label := formatDate(r.At) + " · " + r.Total.String()
		if r.Merchant != "" {
			label += " · " + shorten(r.Merchant, 18)
		}
		rows = append(rows, m.Row(m.Data(label, navReceipt, strconv.FormatInt(r.ID, 10), period)))
	}

	var pager []tele.Btn
	if offset > 0 {
		pager = append(pager, m.Data("←", navHistory, period, strconv.Itoa(max(0, offset-pageSize))))
	}
	if offset+pageSize < total {
		pager = append(pager, m.Data("→", navHistory, period, strconv.Itoa(offset+pageSize)))
	}
	if len(pager) > 0 {
		rows = append(rows, m.Row(pager...))
	}

	rows = append(rows, periodRow(m, navHistory, period))
	rows = append(rows, m.Row(m.Data("← К расходам", navReport, period)))

	m.Inline(rows...)
	return m
}

// receiptKeyboard — возврат из карточки чека обратно в историю.
func receiptKeyboard(period string) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("← К истории", navHistory, period, "0")))
	return m
}

// productKeyboard — возврат из истории цены товара.
func productKeyboard(period string) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("← К расходам", navReport, period)))
	return m
}

// shorten обрезает длинные названия: подпись кнопки ограничена по ширине.
func shorten(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// addedKeyboard даёт перейти из уведомления о добавленном чеке к его составу
// и к общей картине расходов.
func addedKeyboard(receiptID int64) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(
		m.Data("🧾 Состав чека", navReceipt, strconv.FormatInt(receiptID, 10), periodMonth),
		m.Data("📊 Расходы", navReport, periodMonth),
	))
	return m
}
