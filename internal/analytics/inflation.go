package analytics

import (
	"sort"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

// MinPurchases — сколько раз товар должен быть куплен, чтобы попасть
// в постоянную корзину. Случайная разовая покупка индекс только зашумит.
const MinPurchases = 3

// PriceObservation — сводка по одному товару за период.
type PriceObservation struct {
	ProductID  int64
	CategoryID int64
	Title      string
	UnitPrice  domain.Money
	Spend      domain.Money
	Purchases  int
}

type CategoryChange struct {
	CategoryID int64
	Change     float64 // 0.142 означает +14,2%
	Weight     float64
}

type Mover struct {
	ProductID int64
	Title     string
	Before    domain.Money
	After     domain.Money
	Change    float64
}

type Index struct {
	Change     float64
	ByCategory []CategoryChange
	Movers     []Mover
	Matched    int
}

// pair — товар, купленный в обоих периодах.
type pair struct {
	base, now PriceObservation
}

// Inflate считает индекс цен по сопоставимым товарам: взвешенное среднее
// относительных цен, где вес — доля товара в расходах базового периода.
func Inflate(base, current []PriceObservation, minPurchases int) Index {
	matched, totalWeight := matchObservations(base, current, minPurchases)

	idx := Index{Matched: len(matched)}
	if len(matched) == 0 || totalWeight == 0 {
		return idx
	}

	perCategory := map[int64]*weighted{}

	for _, p := range matched {
		ratio := float64(p.now.UnitPrice) / float64(p.base.UnitPrice)
		idx.Change += float64(p.base.Spend) / float64(totalWeight) * ratio

		c, ok := perCategory[p.base.CategoryID]
		if !ok {
			c = &weighted{}
			perCategory[p.base.CategoryID] = c
		}
		c.add(float64(p.base.Spend), ratio)

		idx.Movers = append(idx.Movers, Mover{
			ProductID: p.base.ProductID,
			Title:     p.base.Title,
			Before:    p.base.UnitPrice,
			After:     p.now.UnitPrice,
			Change:    ratio - 1,
		})
	}
	idx.Change--
	idx.ByCategory = categoryChanges(perCategory, totalWeight)

	sort.Slice(idx.Movers, func(i, j int) bool {
		return idx.Movers[i].Change > idx.Movers[j].Change
	})
	return idx
}

// matchObservations отбирает товары, купленные достаточно часто и
// присутствующие в обоих периодах: остальное сравнивать не с чем.
func matchObservations(base, current []PriceObservation, minPurchases int) ([]pair, domain.Money) {
	currentByID := make(map[int64]PriceObservation, len(current))
	for _, o := range current {
		currentByID[o.ProductID] = o
	}

	var matched []pair
	var totalWeight domain.Money

	for _, b := range base {
		if b.Purchases < minPurchases || b.UnitPrice <= 0 {
			continue
		}
		c, ok := currentByID[b.ProductID]
		if !ok || c.UnitPrice <= 0 {
			continue
		}
		matched = append(matched, pair{base: b, now: c})
		totalWeight += b.Spend
	}
	return matched, totalWeight
}

// weighted копит взвешенную сумму отношений цен внутри одной категории.
type weighted struct{ sum, weight float64 }

func (w *weighted) add(weight, ratio float64) {
	w.sum += weight * ratio
	w.weight += weight
}

func categoryChanges(per map[int64]*weighted, totalWeight domain.Money) []CategoryChange {
	out := make([]CategoryChange, 0, len(per))
	for id, c := range per {
		if c.weight == 0 {
			continue
		}
		out = append(out, CategoryChange{
			CategoryID: id,
			Change:     c.sum/c.weight - 1,
			Weight:     c.weight / float64(totalWeight),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Change > out[j].Change })
	return out
}

// TopMovers оставляет n самых подорожавших позиций.
func (i Index) TopMovers(n int) []Mover {
	if len(i.Movers) < n {
		n = len(i.Movers)
	}
	return i.Movers[:n]
}
