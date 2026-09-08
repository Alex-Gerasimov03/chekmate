package bot

import (
	"context"
	"testing"
	"time"
)

func TestTelegramAlive(t *testing.T) {
	var b Bot

	if err := b.TelegramAlive(context.Background()); err == nil {
		t.Error("до первой удачной проверки связь не подтверждена")
	}

	b.lastSeen.Store(time.Now().UnixNano())
	if err := b.TelegramAlive(context.Background()); err != nil {
		t.Errorf("свежая проверка должна считаться живой: %v", err)
	}

	b.lastSeen.Store(time.Now().Add(-telegramStaleAfter - time.Minute).UnixNano())
	if err := b.TelegramAlive(context.Background()); err == nil {
		t.Error("после долгого молчания связь должна считаться потерянной")
	}
}
