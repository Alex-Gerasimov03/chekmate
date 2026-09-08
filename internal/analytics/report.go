package analytics

import (
	"sort"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

// CategorySpend — расходы по категории в текущем и сопоставимом прошлом периоде.
const FallbackCategory = "other"

// PricePoint — одна покупка товара в истории цены.
type PricePoint struct {
	At        string
	Merchant  string
	UnitPrice domain.Money
}

type CategorySpend struct {
	CategoryID int64
	Code       string
	Title      string
	IsFood     bool
	Current    domain.Money
	Previous   domain.Money
}

// Change — относительное изменение расходов. Второе значение равно false,
// когда сравнивать не с чем: в прошлом периоде такой категории не было.
func (c CategorySpend) Change() (float64, bool) {
	if c.Previous == 0 {
		return 0, false
	}
	return float64(c.Current-c.Previous) / float64(c.Previous), true
}

type Report struct {
	Current       Period
	Previous      Period
	Total         domain.Money
	PreviousTotal domain.Money
	Categories    []CategorySpend
	Receipts      int
}

// BuildReport упорядочивает категории по величине трат: пользователю важно
// сперва увидеть, куда ушли основные деньги.
func BuildReport(current, previous Period, rows []CategorySpend, receipts int) Report {
	r := Report{Current: current, Previous: previous, Receipts: receipts}

	r.Categories = make([]CategorySpend, 0, len(rows))
	for _, row := range rows {
		if row.Current == 0 && row.Previous == 0 {
			continue
		}
		r.Categories = append(r.Categories, row)
		r.Total += row.Current
		r.PreviousTotal += row.Previous
	}

	sort.SliceStable(r.Categories, func(i, j int) bool {
		return r.Categories[i].Current > r.Categories[j].Current
	})
	return r
}

// Change — изменение расходов целиком.
func (r Report) Change() (float64, bool) {
	if r.PreviousTotal == 0 {
		return 0, false
	}
	return float64(r.Total-r.PreviousTotal) / float64(r.PreviousTotal), true
}

// FoodShare — доля еды в расходах, узнаваемый показатель уровня жизни.
func (r Report) FoodShare() float64 {
	if r.Total == 0 {
		return 0
	}
	var food domain.Money
	for _, c := range r.Categories {
		if c.IsFood {
			food += c.Current
		}
	}
	return float64(food) / float64(r.Total)
}

// AverageReceipt — средний чек за период.
func (r Report) AverageReceipt() domain.Money {
	if r.Receipts == 0 {
		return 0
	}
	return r.Total / domain.Money(r.Receipts)
}

// Category — запись справочника категорий.
type Category struct {
	ID     int64
	Code   string
	Title  string
	IsFood bool
}

// ProductSpend — сколько потрачено на один товар за период.
type ProductSpend struct {
	ProductID int64
	Title     string
	Spend     domain.Money
	Purchases int
	LastPrice domain.Money
}

// ReceiptSummary — чек в списке истории покупок.
type ReceiptSummary struct {
	ID       int64
	At       time.Time
	Total    domain.Money
	Merchant string
	Items    int
}

// ReceiptLine — позиция конкретного чека.
type ReceiptLine struct {
	RawName   string
	Title     string
	Category  string
	Qty       Quantity
	Sum       domain.Money
	UnitPrice domain.Money
}

// Quantity описывает количество позиции для показа.
type Quantity struct {
	Milli int64
	Unit  string
}
