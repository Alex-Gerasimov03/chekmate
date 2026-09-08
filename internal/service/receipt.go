package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/catalog"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
	"github.com/Alex-Gerasimov03/chekmate/internal/enrich"
	"github.com/Alex-Gerasimov03/chekmate/internal/qr"
)

const (
	SourceQR     = "qr"
	SourceManual = "manual"
)

type BudgetRepo interface {
	GetOrCreate(ctx context.Context, chatID int64, tz string) (int64, error)
}

type ReceiptRepo interface {
	Save(ctx context.Context, budgetID, userID int64, source string, r domain.Receipt) (int64, bool, error)
	AssignProduct(ctx context.Context, receiptID int64, rawName string, productID int64) error
}

type ProductRepo interface {
	SetCategory(ctx context.Context, productID, categoryID int64) error
	FallbackCategoryID(ctx context.Context) (int64, error)
	ListByBudget(ctx context.Context, budgetID int64) ([]catalog.ProductRef, error)
}

type RuleRepo interface {
	Load(ctx context.Context, budgetID int64) ([]catalog.Rule, error)
	Add(ctx context.Context, budgetID, categoryID int64, word string) error
}

// Enricher добывает состав чека. Источники ненадёжны и меняются, поэтому
// он вынесен за интерфейс: чек учитывается и без позиций.
type Enricher interface {
	Enrich(ctx context.Context, r domain.Receipt) (enrich.Result, error)
}

// NoEnricher используется, пока источник позиций не настроен.
type NoEnricher struct{}

func (NoEnricher) Enrich(context.Context, domain.Receipt) (enrich.Result, error) {
	return enrich.Result{}, nil
}

type Service struct {
	budgets  BudgetRepo
	receipts ReceiptRepo
	products ProductRepo
	matcher  *catalog.Matcher
	rules    RuleRepo
	enricher Enricher
	loc      *time.Location
	log      *slog.Logger

	mu   sync.RWMutex
	cats map[int64]*catalog.Categorizer
}

func New(b BudgetRepo, r ReceiptRepo, p ProductRepo, rules RuleRepo, m *catalog.Matcher, e Enricher, loc *time.Location) *Service {
	if e == nil {
		e = NoEnricher{}
	}
	return &Service{
		budgets: b, receipts: r, products: p, rules: rules,
		matcher: m, enricher: e, loc: loc, log: slog.Default(),
		cats: map[int64]*catalog.Categorizer{},
	}
}

// categorizer держит правила бюджета в памяти: их десятки, а читаются они
// на каждой позиции чека.
func (s *Service) categorizer(ctx context.Context, budgetID int64) (*catalog.Categorizer, error) {
	s.mu.RLock()
	c, ok := s.cats[budgetID]
	s.mu.RUnlock()
	if ok {
		return c, nil
	}

	rules, err := s.rules.Load(ctx, budgetID)
	if err != nil {
		return nil, err
	}
	c = catalog.NewCategorizer(rules)

	s.mu.Lock()
	s.cats[budgetID] = c
	s.mu.Unlock()
	return c, nil
}

// ResetRules сбрасывает кэш правил после того, как пользователь добавил своё.
func (s *Service) ResetRules(budgetID int64) {
	s.mu.Lock()
	delete(s.cats, budgetID)
	s.mu.Unlock()
}

// Added — результат добавления траты.
type Added struct {
	ReceiptID int64
	Created   bool // false, если такой чек уже был
	Receipt   domain.Receipt
	Questions []Question
}

// Question — позиция, по которой матчер не уверен и нужен ответ пользователя.
type Question struct {
	ReceiptID  int64
	RawName    string
	Suggestion catalog.Candidate
}

// AddFromQR разбирает строку QR-кода и сохраняет чек.
func (s *Service) AddFromQR(ctx context.Context, chatID, userID int64, raw string) (Added, error) {
	receipt, err := qr.Parse(raw, s.loc)
	if err != nil {
		return Added{}, err
	}

	budgetID, err := s.budgets.GetOrCreate(ctx, chatID, s.loc.String())
	if err != nil {
		return Added{}, err
	}

	// Состав — дополнение к сумме: если источник недоступен или чек ещё не
	// дошёл от кассы, трата всё равно учитывается.
	var items []domain.Item
	if res, err := s.enricher.Enrich(ctx, receipt); err != nil {
		s.log.Warn("состав чека не получен", "err", err)
	} else {
		if res.Merchant != "" {
			receipt.Merchant = res.Merchant
		}
		items = s.itemsFrom(res.Items)
	}

	var questions []Question
	receipt.Items, questions, err = s.resolveItems(ctx, budgetID, items)
	if err != nil {
		return Added{}, err
	}

	id, created, err := s.receipts.Save(ctx, budgetID, userID, SourceQR, receipt)
	if err != nil {
		return Added{}, err
	}
	for i := range questions {
		questions[i].ReceiptID = id
	}
	return Added{ReceiptID: id, Created: created, Receipt: receipt, Questions: questions}, nil
}

// AddManual сохраняет трату, введённую текстом.
func (s *Service) AddManual(ctx context.Context, chatID, userID int64, text string) (Added, error) {
	entry, err := ParseManual(text)
	if err != nil {
		return Added{}, err
	}

	budgetID, err := s.budgets.GetOrCreate(ctx, chatID, s.loc.String())
	if err != nil {
		return Added{}, err
	}

	unitPrice, err := domain.UnitPrice(entry.Sum, entry.Qty)
	if err != nil {
		return Added{}, err
	}

	receipt := domain.Receipt{
		At:        time.Now().In(s.loc),
		Total:     entry.Sum,
		Operation: domain.OpIncome,
		Merchant:  entry.Merchant,
	}

	items, questions, err := s.resolveItems(ctx, budgetID, []domain.Item{{
		RawName: entry.Name, Qty: entry.Qty, Sum: entry.Sum, UnitPrice: unitPrice,
	}})
	if err != nil {
		return Added{}, err
	}
	receipt.Items = items

	id, created, err := s.receipts.Save(ctx, budgetID, userID, SourceManual, receipt)
	if err != nil {
		return Added{}, err
	}
	for i := range questions {
		questions[i].ReceiptID = id
	}
	return Added{ReceiptID: id, Created: created, Receipt: receipt, Questions: questions}, nil
}

// resolveItems сопоставляет позиции с товарами. Позиции, по которым матчер
// не уверен, в чек не попадают: связь уточняется у пользователя отдельно.
func (s *Service) resolveItems(ctx context.Context, budgetID int64, items []domain.Item) ([]domain.Item, []Question, error) {
	if len(items) == 0 {
		return nil, nil, nil
	}

	cats, err := s.categorizer(ctx, budgetID)
	if err != nil {
		return nil, nil, err
	}

	var resolved []domain.Item
	var questions []Question

	for _, it := range items {
		if it.UnitPrice == 0 && !it.Qty.IsZero() {
			price, err := domain.UnitPrice(it.Sum, it.Qty)
			if err != nil {
				return nil, nil, fmt.Errorf("позиция %q: %w", it.RawName, err)
			}
			it.UnitPrice = price
		}

		match, err := s.matcher.Match(ctx, it.RawName)
		if err != nil {
			return nil, nil, err
		}
		if match.Outcome == catalog.OutcomeAmbiguous {
			questions = append(questions, Question{RawName: it.RawName, Suggestion: *match.Suggestion})
			resolved = append(resolved, it) // без товара, но с суммой
			continue
		}
		it.ProductID = match.ProductID

		// Категорию определяем один раз, при заведении товара: дальше она
		// живёт в справочнике и меняется только правкой пользователя.
		if match.Outcome == catalog.OutcomeCreated {
			if err := s.assignCategory(ctx, cats, match.ProductID, match.Result.Canonical); err != nil {
				return nil, nil, err
			}
		}
		resolved = append(resolved, it)
	}
	return resolved, questions, nil
}

// ConfirmProduct привязывает позицию к предложенному товару. Ответ
// запоминается: тот же товар больше не будет спрашиваться.
func (s *Service) ConfirmProduct(ctx context.Context, receiptID int64, rawName string, productID int64) error {
	if err := s.matcher.Confirm(ctx, rawName, productID); err != nil {
		return err
	}
	return s.receipts.AssignProduct(ctx, receiptID, rawName, productID)
}

// RejectSuggestion заводит отдельный товар: пользователь сказал, что
// предложенное — не то же самое.
func (s *Service) RejectSuggestion(ctx context.Context, budgetID, receiptID int64, rawName string) error {
	productID, err := s.matcher.Create(ctx, rawName)
	if err != nil {
		return err
	}

	cats, err := s.categorizer(ctx, budgetID)
	if err != nil {
		return err
	}
	if err := s.assignCategory(ctx, cats, productID, s.matcher.Canonical(rawName)); err != nil {
		return err
	}
	return s.receipts.AssignProduct(ctx, receiptID, rawName, productID)
}

// AddRule сохраняет правило и пересчитывает категории уже купленных товаров:
// иначе оно подействовало бы только на будущие покупки.
func (s *Service) AddRule(ctx context.Context, budgetID, categoryID int64, word string) (int, error) {
	if err := s.rules.Add(ctx, budgetID, categoryID, word); err != nil {
		return 0, err
	}
	s.ResetRules(budgetID)

	cats, err := s.categorizer(ctx, budgetID)
	if err != nil {
		return 0, err
	}

	products, err := s.products.ListByBudget(ctx, budgetID)
	if err != nil {
		return 0, err
	}

	changed := 0
	for _, p := range products {
		categoryID, ok := cats.Categorize(p.Canonical)
		if !ok {
			continue
		}
		if err := s.products.SetCategory(ctx, p.ID, categoryID); err != nil {
			return changed, err
		}
		changed++
	}
	return changed, nil
}

// itemsFrom превращает позиции из внешнего источника в позиции чека.
func (s *Service) itemsFrom(raw []enrich.RawItem) []domain.Item {
	items := make([]domain.Item, 0, len(raw))

	for _, r := range raw {
		qty := domain.NewQuantity(int64(r.Count*1000+0.5), domain.UnitPcs)
		if pack := s.matcher.Parse(r.Name).Qty; !pack.IsZero() {
			qty = pack.Scale(r.Count)
		}

		unitPrice, err := domain.UnitPrice(r.Sum, qty)
		if err != nil {
			continue
		}
		items = append(items, domain.Item{
			RawName: r.Name, Qty: qty, Sum: r.Sum, UnitPrice: unitPrice,
		})
	}
	return items
}

// assignCategory ставит категорию, а неопознанному товару — запасную:
// без категории трата выпала бы из отчёта.
func (s *Service) assignCategory(ctx context.Context, cats *catalog.Categorizer, productID int64, canonical string) error {
	categoryID, ok := cats.Categorize(canonical)
	if !ok {
		var err error
		if categoryID, err = s.products.FallbackCategoryID(ctx); err != nil {
			return err
		}
	}
	return s.products.SetCategory(ctx, productID, categoryID)
}
