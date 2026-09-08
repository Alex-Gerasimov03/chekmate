package bot

import tele "gopkg.in/telebot.v4"

// Кнопки постоянной клавиатуры. Команды набирать неудобно, а чек добавляют
// на ходу, стоя у кассы.
var (
	btnReport    = tele.Btn{Text: "📊 Расходы"}
	btnInflation = tele.Btn{Text: "📈 Инфляция"}
	btnTop       = tele.Btn{Text: "🔺 Подорожало"}
	btnHelp      = tele.Btn{Text: "❓ Помощь"}
)

// mainMenu — клавиатура под полем ввода.
func mainMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{ResizeKeyboard: true}
	m.Reply(
		m.Row(btnReport, btnInflation),
		m.Row(btnTop, btnHelp),
	)
	return m
}

// commands — список для кнопки «Меню» рядом с полем ввода.
func commands() []tele.Command {
	return []tele.Command{
		{Text: "report", Description: "Расходы по категориям за месяц"},
		{Text: "inflation", Description: "Инфляция вашей корзины"},
		{Text: "top", Description: "Что подорожало сильнее всего"},
		{Text: "price", Description: "История цены товара: /price молоко"},
		{Text: "rule", Description: "Своё правило категории"},
		{Text: "help", Description: "Как пользоваться"},
	}
}

// isMenuButton защищает от попытки записать нажатие кнопки как трату,
// если обработчик кнопки почему-то не сработал первым.
func isMenuButton(text string) bool {
	for _, b := range []tele.Btn{btnReport, btnInflation, btnTop, btnHelp} {
		if b.Text == text {
			return true
		}
	}
	return false
}
