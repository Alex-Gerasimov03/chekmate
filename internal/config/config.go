package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	BotToken    string `env:"BOT_TOKEN,required"`
	DatabaseURL string `env:"DATABASE_URL,required"`
	Timezone    string `env:"TIMEZONE" envDefault:"Europe/Moscow"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
	HTTPAddr    string `env:"HTTP_ADDR" envDefault:":8080"`
	// TelegramProxy нужен там, где api.telegram.org недоступен напрямую.
	TelegramProxy string `env:"TELEGRAM_PROXY"`

	// ReceiptAPIToken включает получение состава чека. Без него бот
	// сохраняет только сумму и дату — то, что есть в самом QR-коде.
	ReceiptAPIToken string `env:"RECEIPT_API_TOKEN"`
	ReceiptAPIURL   string `env:"RECEIPT_API_URL" envDefault:"https://proverkacheka.com/api/v1/check/get"`
}

// ReceiptAPIEnabled сообщает, настроено ли получение состава чека.
func (c Config) ReceiptAPIEnabled() bool { return c.ReceiptAPIToken != "" }

func Load() (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return Config{}, err
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return Config{}, fmt.Errorf("часовой пояс %q: %w", c.Timezone, err)
	}
	return c, nil
}

// Location — зона, в которой считается время покупок и границы периодов.
func (c Config) Location() *time.Location {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}
