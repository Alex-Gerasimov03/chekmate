// Package bot — телеграм-интерфейс. Здесь только приём сообщений и вывод:
// вся логика живёт в service и analytics.
package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
	"github.com/Alex-Gerasimov03/chekmate/internal/service"
)

const (
	// requestTimeout ограничивает поход в базу: Telegram всё равно не будет
	// ждать ответа вечно, а зависший запрос держал бы соединение из пула.
	requestTimeout = 15 * time.Second

	// questionTTL — сколько живёт вопрос о принадлежности позиции.
	questionTTL = 30 * time.Minute

	// maxPhotoBytes отсекает снимки, которые незачем разбирать: распознавание
	// прогоняет изображение трижды, и на многомегабайтном файле это заметно.
	maxPhotoBytes = 12 << 20

	// decodeSlots ограничивает число одновременных распознаваний.
	decodeSlots = 2

	// rateLimit — сколько сообщений от одного человека обрабатывается за минуту.
	rateLimit  = 20
	ratePeriod = time.Minute
)

type AnalyticsRepo interface {
	CategorySpends(ctx context.Context, budgetID int64, cur, prev analytics.Period) ([]analytics.CategorySpend, error)
	ReceiptCount(ctx context.Context, budgetID int64, p analytics.Period) (int, error)
	PriceObservations(ctx context.Context, budgetID int64, p analytics.Period) ([]analytics.PriceObservation, error)
	PriceHistory(ctx context.Context, budgetID, productID int64, limit int) ([]analytics.PricePoint, error)
	SearchProducts(ctx context.Context, budgetID int64, query string, limit int) ([]analytics.PriceObservation, error)
	CategoryTitles(ctx context.Context) (map[int64]string, error)

	ProductsInCategory(ctx context.Context, budgetID, categoryID int64, p analytics.Period, limit int) ([]analytics.ProductSpend, error)
	ProductTitle(ctx context.Context, productID int64) (string, error)
	ReceiptsInPeriod(ctx context.Context, budgetID int64, p analytics.Period, limit, offset int) ([]analytics.ReceiptSummary, error)
	CountReceipts(ctx context.Context, budgetID int64, p analytics.Period) (int, error)
	ReceiptLines(ctx context.Context, budgetID, receiptID int64) (analytics.ReceiptSummary, []analytics.ReceiptLine, error)
}

type BudgetRepo interface {
	GetOrCreate(ctx context.Context, chatID int64, tz string) (int64, error)
}

type CategoryRepo interface {
	ByCode(ctx context.Context, code string) (int64, error)
	All(ctx context.Context) ([]analytics.Category, error)
}

type Bot struct {
	tb      *tele.Bot
	svc     *service.Service
	stats   AnalyticsRepo
	budgets BudgetRepo
	cats    CategoryRepo
	loc     *time.Location
	log     *slog.Logger

	pending *pendingStore
	decode  chan struct{}
	limiter *limiter

	// lastSeen — время последнего успешного ответа Telegram.
	lastSeen atomic.Int64

	// base отменяется при остановке: начатые запросы не переживают выключение.
	base context.Context
}

var (
	btnYes = &tele.Btn{Unique: "same_product"}
	btnNo  = &tele.Btn{Unique: "other_product"}
)

func New(token, proxyURL string, svc *service.Service, stats AnalyticsRepo, budgets BudgetRepo, cats CategoryRepo, loc *time.Location, log *slog.Logger) (*Bot, error) {
	client, err := httpClient(proxyURL)
	if err != nil {
		return nil, err
	}

	tb, err := tele.NewBot(tele.Settings{
		Token:  token,
		Client: client,
		// Long polling не требует домена и сертификата: бот поднимается
		// хоть за NAT, а вебхуки нужны только под нагрузкой.
		Poller:    &tele.LongPoller{Timeout: 10 * time.Second},
		ParseMode: tele.ModeHTML,
	})
	if err != nil {
		return nil, fmt.Errorf("подключение к Telegram: %w", err)
	}

	b := &Bot{
		tb: tb, svc: svc, stats: stats, budgets: budgets, cats: cats,
		loc: loc, log: log,
		pending: newPendingStore(questionTTL),
		decode:  make(chan struct{}, decodeSlots),
		limiter: newLimiter(rateLimit, ratePeriod),
		base:    context.Background(),
	}
	b.routes()
	return b, nil
}

func (b *Bot) routes() {
	b.tb.Use(b.throttle)

	// Меню команд ставится один раз при запуске; ошибка не критична —
	// бот работает и без него.
	if err := b.tb.SetCommands(commands()); err != nil {
		b.log.Warn("не удалось установить меню команд", "err", err)
	}

	b.tb.Handle("/start", b.onStart)
	b.tb.Handle("/help", b.onStart)
	b.tb.Handle("/report", b.onReport)
	b.tb.Handle("/inflation", b.onInflation)
	b.tb.Handle("/price", b.onPrice)
	b.tb.Handle("/rule", b.onRule)
	b.tb.Handle("/top", b.onTop)

	b.tb.Handle(&btnReport, b.onReport)
	b.tb.Handle(&btnInflation, b.onInflation)
	b.tb.Handle(&btnTop, b.onTop)
	b.tb.Handle(&btnHelp, b.onStart)

	b.tb.Handle(&tele.Btn{Unique: navReport}, b.onNavReport)
	b.tb.Handle(&tele.Btn{Unique: navCategory}, b.onNavCategory)
	b.tb.Handle(&tele.Btn{Unique: navProduct}, b.onNavProduct)
	b.tb.Handle(&tele.Btn{Unique: navHistory}, b.onNavHistory)
	b.tb.Handle(&tele.Btn{Unique: navReceipt}, b.onNavReceipt)
	b.tb.Handle(tele.OnPhoto, b.onPhoto)
	b.tb.Handle(tele.OnText, b.onText)
	b.tb.Handle(btnYes, b.onSameProduct)
	b.tb.Handle(btnNo, b.onOtherProduct)
}

// Start блокируется до отмены контекста и останавливает бота корректно.
func (b *Bot) Start(ctx context.Context) {
	b.base = ctx
	go func() {
		<-ctx.Done()
		b.tb.Stop()
	}()
	go b.watchTelegram(ctx)
	b.tb.Start()
}

// throttle отсекает поток сообщений от одного пользователя.
func (b *Bot) throttle(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		if c.Sender() == nil || b.limiter.allow(c.Sender().ID) {
			return next(c)
		}
		b.log.Warn("превышен лимит сообщений", "user", c.Sender().ID)
		return nil
	}
}

// budget возвращает бюджет чата, заводя его при первом обращении.
func (b *Bot) budget(ctx context.Context, c tele.Context) (int64, error) {
	return b.budgets.GetOrCreate(ctx, c.Chat().ID, b.loc.String())
}

func (b *Bot) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(b.base, requestTimeout)
}

const help = `Считаю вашу личную инфляцию по чекам.

Пришлите фото QR-кода с чека — я запишу трату.
Или напишите текстом: <code>молоко 930мл 89</code>

Кнопки под полем ввода — то же самое, что команды:

/report — расходы по категориям и сравнение с прошлым месяцем
/inflation — насколько подорожала лично ваша корзина
/top — что подорожало сильнее всего
/price молоко — история цены товара
/rule сгущенка sweets — своё правило категории

В группе я реагирую только на команды, фото и ответы на свои сообщения —
чтобы не мешать переписке.`

func (b *Bot) onStart(c tele.Context) error { return c.Send(help, mainMenu()) }

// addressed отвечает, обращались ли к боту. В группе бот молчит, пока к нему
// не обратились: иначе он реагировал бы на каждую реплику семейной переписки.
func (b *Bot) addressed(c tele.Context) bool {
	if c.Chat() == nil || c.Chat().Type == tele.ChatPrivate {
		return true
	}
	msg := c.Message()
	if msg == nil {
		return false
	}
	if msg.ReplyTo != nil && msg.ReplyTo.Sender != nil && msg.ReplyTo.Sender.IsBot {
		return true
	}
	return strings.Contains(msg.Text, "@"+b.username())
}

func (b *Bot) username() string {
	if me := b.tb.Me; me != nil {
		return me.Username
	}
	return ""
}
