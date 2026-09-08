package bot

import (
	"strings"

	tele "gopkg.in/telebot.v4"
)

// edit переписывает сообщение на месте и гасит «часики» на кнопке.
func (b *Bot) edit(c tele.Context, text string, markup *tele.ReplyMarkup) error {
	err := c.Edit(text, markup)
	if err != nil && !isNotModified(err) {
		b.log.Error("не удалось обновить сообщение", "err", err)
		return c.Respond(&tele.CallbackResponse{Text: "Не получилось обновить"})
	}
	return c.Respond()
}

func isNotModified(err error) bool {
	return err != nil && strings.Contains(err.Error(), "message is not modified")
}
