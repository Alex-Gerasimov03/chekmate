package bot

import (
	"context"
	"fmt"
	"time"
)

const (
	// telegramProbeEvery — как часто бот убеждается, что Telegram отвечает.
	telegramProbeEvery = time.Minute

	// telegramStaleAfter — после какого молчания считать связь потерянной.
	// Порог с запасом: одна неудачная проверка ещё не повод поднимать тревогу.
	telegramStaleAfter = 5 * time.Minute
)

// watchTelegram отмечает время последнего успешного ответа Telegram.
//
// Без этого readyz оставался зелёным при упавшем прокси: база отвечает,
// а бот сообщений не получает и починить себя не может.
func (b *Bot) watchTelegram(ctx context.Context) {
	b.probeTelegram()

	ticker := time.NewTicker(telegramProbeEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.probeTelegram()
		}
	}
}

func (b *Bot) probeTelegram() {
	if _, err := b.tb.Raw("getMe", nil); err != nil {
		b.log.Warn("Telegram не отвечает", "err", err)
		return
	}
	b.lastSeen.Store(time.Now().UnixNano())
}

// TelegramAlive сообщает, отвечал ли Telegram недавно.
func (b *Bot) TelegramAlive(context.Context) error {
	last := b.lastSeen.Load()
	if last == 0 {
		return fmt.Errorf("связь с Telegram ещё не подтверждена")
	}
	if since := time.Since(time.Unix(0, last)); since > telegramStaleAfter {
		return fmt.Errorf("нет ответа от Telegram %s", since.Truncate(time.Second))
	}
	return nil
}
