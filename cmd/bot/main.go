package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alex-Gerasimov03/chekmate/internal/bot"
	"github.com/Alex-Gerasimov03/chekmate/internal/catalog"
	"github.com/Alex-Gerasimov03/chekmate/internal/config"
	"github.com/Alex-Gerasimov03/chekmate/internal/enrich"
	"github.com/Alex-Gerasimov03/chekmate/internal/httpsrv"
	"github.com/Alex-Gerasimov03/chekmate/internal/normalize"
	"github.com/Alex-Gerasimov03/chekmate/internal/service"
	"github.com/Alex-Gerasimov03/chekmate/internal/storage"
	"github.com/Alex-Gerasimov03/chekmate/internal/storage/postgres"
)

func main() {
	if err := run(); err != nil {
		slog.Error("остановлено с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level(cfg.LogLevel)}))
	slog.SetDefault(log)

	// Контекст отменяется по сигналу: Telegram отпускается, соединения
	// закрываются, начатые обработки успевают завершиться.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := storage.Migrate(ctx, cfg.DatabaseURL); err != nil {
		return err
	}

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	matcher, err := prepareCatalog(ctx, pool, log)
	if err != nil {
		return err
	}

	svc := service.New(
		postgres.NewBudgets(pool), postgres.NewReceipts(pool), postgres.NewProducts(pool),
		postgres.NewRules(pool), matcher, newEnricher(cfg, log), cfg.Location(),
	)

	log.Info("прокси для Telegram", "адрес", cfg.TelegramProxy)
	b, err := bot.New(cfg.BotToken, cfg.TelegramProxy, svc,
		postgres.NewAnalytics(pool), postgres.NewBudgets(pool), postgres.NewCategories(pool),
		cfg.Location(), log)
	if err != nil {
		return err
	}

	// Готовность — это и база, и связь с Telegram: при упавшем прокси бот
	// сообщений не получает, хотя база отвечает.
	checks := map[string]func(context.Context) error{
		"база":     pool.Ping,
		"telegram": b.TelegramAlive,
	}
	go httpsrv.New(cfg.HTTPAddr, checks, log).Run(ctx)

	log.Info("бот запущен", "часовой пояс", cfg.Timezone, "служебный http", cfg.HTTPAddr)
	b.Start(ctx)
	log.Info("бот остановлен")
	return nil
}

// prepareCatalog пересчитывает ключи товаров под текущий словарь и раздаёт
// категории тем, кого раньше опознать не удалось.
func prepareCatalog(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) (*catalog.Matcher, error) {
	dict, err := postgres.NewDictionaries(pool).Load(ctx)
	if err != nil {
		return nil, err
	}
	log.Info("словарь загружен",
		"сокращений", len(dict.Abbrev), "стоп-слов", len(dict.StopWords), "версия", dict.Version)

	products := postgres.NewProducts(pool)
	norm := normalize.New(dict)

	// Словарь мог пополниться с прошлого запуска: без пересчёта один товар
	// разъедется на две записи и история цен по нему порвётся.
	updated, merged, err := products.Restem(ctx, norm)
	if err != nil {
		return nil, err
	}
	if updated+merged > 0 {
		log.Info("товары пересчитаны под новый словарь", "обновлено", updated, "слито", merged)
	}

	commonRules, err := postgres.NewRules(pool).Load(ctx, 0)
	if err != nil {
		return nil, err
	}
	changed, unknown, err := products.Recategorize(ctx, catalog.NewCategorizer(commonRules))
	if err != nil {
		return nil, err
	}
	if changed > 0 {
		log.Info("категории пересмотрены", "изменено", changed, "не опознано", unknown)
	}

	return catalog.NewMatcher(products, norm), nil
}

// newEnricher включает получение состава чека, если задан ключ. Без него бот
// учитывает только сумму и дату из QR-кода.
func newEnricher(cfg config.Config, log *slog.Logger) service.Enricher {
	if !cfg.ReceiptAPIEnabled() {
		log.Warn("RECEIPT_API_TOKEN не задан: состав чеков получать неоткуда, " +
			"будут учитываться только суммы")
		return nil
	}
	log.Info("получение состава чека включено", "источник", cfg.ReceiptAPIURL)
	return enrich.New(cfg.ReceiptAPIURL, cfg.ReceiptAPIToken, 0)
}

func level(s string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return l
}
