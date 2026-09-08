package analytics

import (
	"math"
	"testing"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

func obs(id int64, price, spend string, purchases int) PriceObservation {
	return PriceObservation{
		ProductID:  id,
		CategoryID: 1,
		UnitPrice:  domain.MustParseMoney(price),
		Spend:      domain.MustParseMoney(spend),
		Purchases:  purchases,
	}
}

func assertChange(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("изменение %.4f, ожидалось %.4f", got, want)
	}
}

func TestInflateNoChange(t *testing.T) {
	base := []PriceObservation{obs(1, "100.00", "500.00", 5)}
	current := []PriceObservation{obs(1, "100.00", "500.00", 5)}

	assertChange(t, Inflate(base, current, MinPurchases).Change, 0)
}

func TestInflateSingleProduct(t *testing.T) {
	base := []PriceObservation{obs(1, "100.00", "500.00", 5)}
	current := []PriceObservation{obs(1, "120.00", "600.00", 5)}

	idx := Inflate(base, current, MinPurchases)
	assertChange(t, idx.Change, 0.2)
	if idx.Matched != 1 {
		t.Errorf("сопоставлено %d товаров, ожидался 1", idx.Matched)
	}
}

// Вес по расходам — суть индекса: товар, на который уходит десятая часть
// корзины, не может двигать её так же сильно, как основной.
func TestInflateWeightsBySpend(t *testing.T) {
	base := []PriceObservation{
		obs(1, "100.00", "900.00", 5), // 90% корзины
		obs(2, "100.00", "100.00", 5), // 10% корзины
	}
	current := []PriceObservation{
		obs(1, "100.00", "900.00", 5), // не изменился
		obs(2, "200.00", "200.00", 5), // подорожал вдвое
	}

	// 0,9*1,0 + 0,1*2,0 = 1,1
	assertChange(t, Inflate(base, current, MinPurchases).Change, 0.1)
}

// Рост расходов из-за того, что стали больше покупать, — не инфляция.
func TestInflateIgnoresVolumeGrowth(t *testing.T) {
	base := []PriceObservation{obs(1, "100.00", "500.00", 5)}
	current := []PriceObservation{obs(1, "100.00", "5000.00", 50)}

	assertChange(t, Inflate(base, current, MinPurchases).Change, 0)
}

// Товар, купленный только в одном периоде, сравнивать не с чем.
func TestInflateSkipsUnmatched(t *testing.T) {
	base := []PriceObservation{obs(1, "100.00", "500.00", 5), obs(2, "50.00", "500.00", 5)}
	current := []PriceObservation{obs(1, "150.00", "750.00", 5)}

	idx := Inflate(base, current, MinPurchases)
	if idx.Matched != 1 {
		t.Errorf("сопоставлено %d товаров, ожидался 1", idx.Matched)
	}
	assertChange(t, idx.Change, 0.5)
}

// Разовая покупка не должна попадать в постоянную корзину.
func TestInflateRespectsPurchaseThreshold(t *testing.T) {
	base := []PriceObservation{obs(1, "100.00", "500.00", 2)}
	current := []PriceObservation{obs(1, "300.00", "1500.00", 2)}

	idx := Inflate(base, current, MinPurchases)
	if idx.Matched != 0 {
		t.Errorf("сопоставлено %d товаров, ожидалось 0", idx.Matched)
	}
	assertChange(t, idx.Change, 0)
}

func TestInflateByCategory(t *testing.T) {
	base := []PriceObservation{
		{ProductID: 1, CategoryID: 10, UnitPrice: domain.MustParseMoney("100.00"), Spend: domain.MustParseMoney("500.00"), Purchases: 5},
		{ProductID: 2, CategoryID: 20, UnitPrice: domain.MustParseMoney("100.00"), Spend: domain.MustParseMoney("500.00"), Purchases: 5},
	}
	current := []PriceObservation{
		{ProductID: 1, CategoryID: 10, UnitPrice: domain.MustParseMoney("140.00"), Purchases: 5},
		{ProductID: 2, CategoryID: 20, UnitPrice: domain.MustParseMoney("90.00"), Purchases: 5},
	}

	idx := Inflate(base, current, MinPurchases)
	if len(idx.ByCategory) != 2 {
		t.Fatalf("категорий %d, ожидалось 2", len(idx.ByCategory))
	}
	// Категории отсортированы по убыванию роста.
	if idx.ByCategory[0].CategoryID != 10 {
		t.Errorf("первой должна идти подорожавшая категория, получена %d", idx.ByCategory[0].CategoryID)
	}
	assertChange(t, idx.ByCategory[0].Change, 0.4)
	assertChange(t, idx.ByCategory[1].Change, -0.1)
}

func TestInflateMoversSortedByGrowth(t *testing.T) {
	base := []PriceObservation{obs(1, "100.00", "300.00", 5), obs(2, "100.00", "300.00", 5)}
	current := []PriceObservation{obs(1, "110.00", "330.00", 5), obs(2, "150.00", "450.00", 5)}

	movers := Inflate(base, current, MinPurchases).TopMovers(1)
	if len(movers) != 1 {
		t.Fatalf("получено %d позиций", len(movers))
	}
	if movers[0].ProductID != 2 {
		t.Errorf("первым должен идти товар 2, получен %d", movers[0].ProductID)
	}
	assertChange(t, movers[0].Change, 0.5)
}

func TestInflateEmpty(t *testing.T) {
	idx := Inflate(nil, nil, MinPurchases)
	if idx.Matched != 0 || idx.Change != 0 {
		t.Errorf("на пустых данных индекс должен быть нулевым: %+v", idx)
	}
}
