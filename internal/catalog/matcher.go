package catalog

import (
	"context"
	"fmt"
	"sync"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
	"github.com/Alex-Gerasimov03/chekmate/internal/normalize"
)

// Пороги подобраны так, чтобы бот не дёргал вопросами по мелочи, но и не
// склеивал молча разные товары.
const (
	confidentSimilarity = 0.6
	askSimilarity       = 0.4
)

type Candidate struct {
	ProductID  int64
	Title      string
	Similarity float64
}

// ProductRef — товар, встречавшийся в чеках бюджета.
type ProductRef struct {
	ID        int64
	Canonical string
}

type NewProduct struct {
	Canonical string
	Title     string
	Fat       int
	Unit      domain.Unit
	PackMilli int64
	Version   int
}

type Repo interface {
	AliasProduct(ctx context.Context, raw string) (int64, bool, error)
	FindSimilar(ctx context.Context, canonical string, unit domain.Unit, fat int) ([]Candidate, error)
	CreateProduct(ctx context.Context, p NewProduct) (int64, error)
	SaveAlias(ctx context.Context, raw string, productID int64, confirmed bool) error
}

type Outcome int

const (
	OutcomeAlias     Outcome = iota // строка уже встречалась
	OutcomeMatched                  // нашёлся похожий товар
	OutcomeAmbiguous                // похоже, но нужно подтверждение
	OutcomeCreated                  // товар заведён впервые
)

type Match struct {
	ProductID  int64
	Outcome    Outcome
	Result     normalize.Result
	Suggestion *Candidate // заполнено только при OutcomeAmbiguous
}

// Matcher связывает строку чека с товаром.
type Matcher struct {
	repo Repo
	norm *normalize.Normalizer

	mu    sync.RWMutex
	cache map[string]int64
}

func NewMatcher(repo Repo, norm *normalize.Normalizer) *Matcher {
	return &Matcher{repo: repo, norm: norm, cache: map[string]int64{}}
}

func (m *Matcher) Match(ctx context.Context, raw string) (Match, error) {
	res := m.norm.Name(raw)

	m.mu.RLock()
	id, hit := m.cache[raw]
	m.mu.RUnlock()
	if hit {
		return Match{ProductID: id, Outcome: OutcomeAlias, Result: res}, nil
	}

	id, found, err := m.repo.AliasProduct(ctx, raw)
	if err != nil {
		return Match{}, fmt.Errorf("поиск по алиасам: %w", err)
	}
	if found {
		m.remember(raw, id)
		return Match{ProductID: id, Outcome: OutcomeAlias, Result: res}, nil
	}

	if res.Canonical == "" {
		return Match{}, fmt.Errorf("из %q не удалось получить название товара", raw)
	}

	candidates, err := m.repo.FindSimilar(ctx, res.Canonical, unitOf(res), res.Fat)
	if err != nil {
		return Match{}, fmt.Errorf("поиск похожих товаров: %w", err)
	}

	if len(candidates) > 0 {
		best := candidates[0]
		switch {
		case best.Similarity >= confidentSimilarity:
			if err := m.bind(ctx, raw, best.ProductID, false); err != nil {
				return Match{}, err
			}
			return Match{ProductID: best.ProductID, Outcome: OutcomeMatched, Result: res}, nil

		case best.Similarity >= askSimilarity:
			// Алиас не пишем: пока пользователь не подтвердил, связь неизвестна.
			return Match{Outcome: OutcomeAmbiguous, Result: res, Suggestion: &best}, nil
		}
	}

	id, err = m.repo.CreateProduct(ctx, newProductFrom(res, m.norm.Version()))
	if err != nil {
		return Match{}, fmt.Errorf("создание товара: %w", err)
	}
	if err := m.bind(ctx, raw, id, false); err != nil {
		return Match{}, err
	}
	return Match{ProductID: id, Outcome: OutcomeCreated, Result: res}, nil
}

// Confirm закрепляет выбор пользователя: с этого момента строка привязана
// к товару навсегда и вопрос больше не задаётся.
func (m *Matcher) Confirm(ctx context.Context, raw string, productID int64) error {
	return m.bind(ctx, raw, productID, true)
}

func (m *Matcher) bind(ctx context.Context, raw string, productID int64, confirmed bool) error {
	if err := m.repo.SaveAlias(ctx, raw, productID, confirmed); err != nil {
		return fmt.Errorf("сохранение алиаса: %w", err)
	}
	m.remember(raw, productID)
	return nil
}

func (m *Matcher) remember(raw string, productID int64) {
	m.mu.Lock()
	m.cache[raw] = productID
	m.mu.Unlock()
}

// Create заводит товар принудительно — когда пользователь отверг подсказку.
func (m *Matcher) Create(ctx context.Context, raw string) (int64, error) {
	res := m.norm.Name(raw)
	if res.Canonical == "" {
		return 0, fmt.Errorf("из %q не удалось получить название товара", raw)
	}

	id, err := m.repo.CreateProduct(ctx, newProductFrom(res, m.norm.Version()))
	if err != nil {
		return 0, fmt.Errorf("создание товара: %w", err)
	}
	if err := m.bind(ctx, raw, id, true); err != nil {
		return 0, err
	}
	return id, nil
}

// unitOf подставляет штуки, когда в названии нет ни веса, ни объёма:
// "Бананы вес" и "хлеб" — обычные товары, и единица у них должна быть.
func unitOf(res normalize.Result) domain.Unit {
	if u := res.Qty.Unit(); u != "" {
		return u
	}
	return domain.UnitPcs
}

func newProductFrom(res normalize.Result, version int) NewProduct {
	return NewProduct{
		Canonical: res.Canonical,
		Title:     res.Title,
		Fat:       res.Fat,
		Unit:      unitOf(res),
		PackMilli: res.Qty.Milli(),
		Version:   version,
	}
}

// Canonical отдаёт канонический ключ строки — нужен для категоризации.
func (m *Matcher) Canonical(raw string) string { return m.norm.Name(raw).Canonical }

// Parse отдаёт разобранное название: сервису нужны количество и жирность,
// вытащенные из строки чека.
func (m *Matcher) Parse(raw string) normalize.Result { return m.norm.Name(raw) }
