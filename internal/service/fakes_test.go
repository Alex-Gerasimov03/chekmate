package service

import (
	"context"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/catalog"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
	"github.com/Alex-Gerasimov03/chekmate/internal/normalize"
)

// Заглушки хранилища: сценарии проверяются без базы, работа самих запросов —
// отдельно, в тестах репозиториев.

type fakeBudgets struct{ id int64 }

func (f *fakeBudgets) GetOrCreate(context.Context, int64, string) (int64, error) {
	if f.id == 0 {
		f.id = 1
	}
	return f.id, nil
}

type savedReceipt struct {
	budgetID int64
	source   string
	receipt  domain.Receipt
}

type fakeReceipts struct {
	saved    []savedReceipt
	byKey    map[string]int64
	assigned map[string]int64
	nextID   int64
}

func newFakeReceipts() *fakeReceipts {
	return &fakeReceipts{byKey: map[string]int64{}, assigned: map[string]int64{}}
}

func (f *fakeReceipts) Save(_ context.Context, budgetID, _ int64, source string, r domain.Receipt) (int64, bool, error) {
	if r.IsFiscal() {
		if id, ok := f.byKey[r.FiscalKey()]; ok {
			return id, false, nil
		}
	}
	f.nextID++
	if r.IsFiscal() {
		f.byKey[r.FiscalKey()] = f.nextID
	}
	f.saved = append(f.saved, savedReceipt{budgetID: budgetID, source: source, receipt: r})
	return f.nextID, true, nil
}

func (f *fakeReceipts) AssignProduct(_ context.Context, _ int64, rawName string, productID int64) error {
	f.assigned[rawName] = productID
	return nil
}

type fakeProducts struct {
	categories map[int64]int64
	byBudget   []catalog.ProductRef
}

func (f *fakeProducts) SetCategory(_ context.Context, productID, categoryID int64) error {
	if f.categories == nil {
		f.categories = map[int64]int64{}
	}
	f.categories[productID] = categoryID
	return nil
}

func (f *fakeProducts) FallbackCategoryID(context.Context) (int64, error) { return other, nil }

func (f *fakeProducts) ListByBudget(context.Context, int64) ([]catalog.ProductRef, error) {
	return f.byBudget, nil
}

type fakeRules struct{ rules []catalog.Rule }

func (f *fakeRules) Load(context.Context, int64) ([]catalog.Rule, error) { return f.rules, nil }

func (f *fakeRules) Add(_ context.Context, _, categoryID int64, word string) error {
	f.rules = append(f.rules, catalog.Rule{Word: word, CategoryID: categoryID, Personal: true})
	return nil
}

// fakeCatalog — хранилище товаров для матчера.
type fakeCatalog struct {
	aliases  map[string]int64
	similar  []catalog.Candidate
	products map[int64]catalog.NewProduct
	nextID   int64
}

func newFakeCatalog() *fakeCatalog {
	return &fakeCatalog{aliases: map[string]int64{}, products: map[int64]catalog.NewProduct{}}
}

func (f *fakeCatalog) AliasProduct(_ context.Context, raw string) (int64, bool, error) {
	id, ok := f.aliases[raw]
	return id, ok, nil
}

func (f *fakeCatalog) FindSimilar(context.Context, string, domain.Unit, int) ([]catalog.Candidate, error) {
	return f.similar, nil
}

func (f *fakeCatalog) CreateProduct(_ context.Context, p catalog.NewProduct) (int64, error) {
	f.nextID++
	f.products[f.nextID] = p
	return f.nextID, nil
}

func (f *fakeCatalog) SaveAlias(_ context.Context, raw string, productID int64, _ bool) error {
	f.aliases[raw] = productID
	return nil
}

type env struct {
	svc      *Service
	budgets  *fakeBudgets
	receipts *fakeReceipts
	products *fakeProducts
	rules    *fakeRules
	catalog  *fakeCatalog
}

const (
	dairy   = int64(1)
	grocery = int64(2)
	other   = int64(99)
)

func newEnv() *env {
	dict := normalize.Dictionary{
		Abbrev:    map[string]string{"мол": "молоко", "простокв": "простоквашино"},
		StopWords: map[string]bool{"вес": true},
		Version:   1,
	}

	e := &env{
		budgets:  &fakeBudgets{},
		receipts: newFakeReceipts(),
		products: &fakeProducts{},
		rules:    &fakeRules{rules: []catalog.Rule{{Word: "молоко", CategoryID: dairy}}},
		catalog:  newFakeCatalog(),
	}
	matcher := catalog.NewMatcher(e.catalog, normalize.New(dict))
	e.svc = New(e.budgets, e.receipts, e.products, e.rules, matcher, nil, time.UTC)
	return e
}
