package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
	"github.com/Alex-Gerasimov03/chekmate/internal/catalog"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

// seedPurchase кладёт чек с одной позицией нужного товара и датой.
func seedPurchase(t *testing.T, budgetID, productID int64, at time.Time, sum, unitPrice string) {
	t.Helper()
	ctx := context.Background()

	var receiptID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO receipts (budget_id, user_id, total, bought_at, merchant, source)
		VALUES ($1, 1, $2, $3, 'Тест', 'qr') RETURNING id`,
		budgetID, int64(domain.MustParseMoney(sum)), at).Scan(&receiptID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO receipt_items (receipt_id, raw_name, product_id, qty_milli, unit, sum, unit_price)
		VALUES ($1, 'тест', $2, 1000, 'kg', $3, $4)`,
		receiptID, productID, int64(domain.MustParseMoney(sum)), int64(domain.MustParseMoney(unitPrice)))
	if err != nil {
		t.Fatal(err)
	}
}

func seedProduct(t *testing.T, canonical, categoryCode string) int64 {
	t.Helper()
	ctx := context.Background()

	id, err := NewProducts(pool).CreateProduct(ctx, catalog.NewProduct{
		Canonical: canonical, Title: canonical,
		Unit: domain.UnitKg, PackMilli: 1000, Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	var categoryID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM categories WHERE code = $1`, categoryCode).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err := NewProducts(pool).SetCategory(ctx, id, categoryID); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAnalyticsCategorySpends(t *testing.T) {
	ctx := context.Background()
	budget := newBudget(t)
	product := seedProduct(t, "масл тест "+time.Now().Format("150405.000"), "dairy")

	cur := analytics.Period{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)}
	prev := analytics.Period{From: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)}

	seedPurchase(t, budget, product, time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC), "300.00", "300.00")
	seedPurchase(t, budget, product, time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC), "200.00", "200.00")
	// Покупка вне обоих периодов не должна попасть ни в один столбец.
	seedPurchase(t, budget, product, time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC), "999.00", "999.00")

	rows, err := NewAnalytics(pool).CategorySpends(ctx, budget, cur, prev)
	if err != nil {
		t.Fatal(err)
	}

	report := analytics.BuildReport(cur, prev, rows, 2)
	if want := domain.MustParseMoney("300.00"); report.Total != want {
		t.Errorf("текущие траты %s, ожидалось %s", report.Total, want)
	}
	if want := domain.MustParseMoney("200.00"); report.PreviousTotal != want {
		t.Errorf("прошлые траты %s, ожидалось %s", report.PreviousTotal, want)
	}
}

// Медиана защищает индекс от разовой акционной цены.
func TestAnalyticsPriceObservationsUsesMedian(t *testing.T) {
	ctx := context.Background()
	budget := newBudget(t)
	product := seedProduct(t, "сыр тест "+time.Now().Format("150405.000"), "dairy")

	period := analytics.Period{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	for _, price := range []string{"100.00", "100.00", "40.00"} { // третья — по акции
		seedPurchase(t, budget, product, time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC), price, price)
	}

	got, err := NewAnalytics(pool).PriceObservations(ctx, budget, period)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("товаров %d, ожидался 1", len(got))
	}
	if want := domain.MustParseMoney("100.00"); got[0].UnitPrice != want {
		t.Errorf("медианная цена %s, ожидалась %s", got[0].UnitPrice, want)
	}
	if got[0].Purchases != 3 {
		t.Errorf("покупок %d, ожидалось 3", got[0].Purchases)
	}
	if want := domain.MustParseMoney("240.00"); got[0].Spend != want {
		t.Errorf("расходы %s, ожидались %s", got[0].Spend, want)
	}
}

func TestAnalyticsPriceHistoryAndSearch(t *testing.T) {
	ctx := context.Background()
	budget := newBudget(t)
	title := "кофе тест " + time.Now().Format("150405.000")
	product := seedProduct(t, title, "coffee_tea")

	seedPurchase(t, budget, product, time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC), "300.00", "300.00")
	seedPurchase(t, budget, product, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC), "390.00", "390.00")

	repo := NewAnalytics(pool)
	history, err := repo.PriceHistory(ctx, budget, product, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("точек истории %d, ожидалось 2", len(history))
	}
	if want := domain.MustParseMoney("390.00"); history[0].UnitPrice != want {
		t.Errorf("первой должна идти свежая цена %s, получена %s", want, history[0].UnitPrice)
	}

	found, err := repo.SearchProducts(ctx, budget, "кофе тест", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 || found[0].ProductID != product {
		t.Errorf("поиск не нашёл товар: %+v", found)
	}
}

// Итог отчёта обязан сходиться с суммой чеков: трата, которую не удалось
// разложить по позициям, всё равно потрачена и должна быть видна.
func TestCategorySpendsAccountForEverything(t *testing.T) {
	ctx := context.Background()
	budget := newBudget(t)
	stamp := "z" + time.Now().Format("150405")
	product := seedProduct(t, "чай тест "+stamp, "coffee_tea")

	at := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	cur := analytics.Period{
		From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	prev := analytics.Period{From: cur.From.AddDate(0, -1, 0), To: cur.From}

	// Чек с разложенным составом.
	seedPurchase(t, budget, product, at, "300.00", "300.00")

	// Чек, состав которого получить не удалось: только сумма.
	_, err := pool.Exec(ctx, `
		INSERT INTO receipts (budget_id, user_id, total, bought_at, merchant, source)
		VALUES ($1, 1, $2, $3, 'Без состава', 'qr')`,
		budget, int64(domain.MustParseMoney("1000.00")), at)
	if err != nil {
		t.Fatal(err)
	}

	rows, err := NewAnalytics(pool).CategorySpends(ctx, budget, cur, prev)
	if err != nil {
		t.Fatal(err)
	}

	report := analytics.BuildReport(cur, prev, rows, 2)
	if want := domain.MustParseMoney("1300.00"); report.Total != want {
		t.Errorf("итого %s, ожидалось %s — часть трат потерялась", report.Total, want)
	}

	var fallback domain.Money
	for _, c := range report.Categories {
		if c.Code == analytics.FallbackCategory {
			fallback = c.Current
		}
	}
	if want := domain.MustParseMoney("1000.00"); fallback != want {
		t.Errorf("в прочем %s, ожидалось %s", fallback, want)
	}
}
