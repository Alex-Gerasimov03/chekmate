package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

// период, в который попадают все подготовленные покупки
func testPeriod() analytics.Period {
	return analytics.Period{
		From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestProductsInCategory(t *testing.T) {
	ctx := context.Background()
	budget := newBudget(t)
	stamp := time.Now().Format("150405")

	milk := seedProduct(t, "молок тест "+stamp, "dairy")
	cheese := seedProduct(t, "сыр тест "+stamp, "dairy")
	beef := seedProduct(t, "говядин тест "+stamp, "meat")

	at := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	seedPurchase(t, budget, milk, at, "89.00", "95.69")
	seedPurchase(t, budget, milk, at, "89.00", "95.69")
	seedPurchase(t, budget, cheese, at, "400.00", "2000.00")
	seedPurchase(t, budget, beef, at, "700.00", "700.00")

	var dairyID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM categories WHERE code = 'dairy'`).Scan(&dairyID); err != nil {
		t.Fatal(err)
	}

	items, err := NewAnalytics(pool).ProductsInCategory(ctx, budget, dairyID, testPeriod(), 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Fatalf("товаров %d, ожидалось 2 (мясо не должно попасть)", len(items))
	}
	// Сортировка по сумме: сыр дороже двух пакетов молока.
	if items[0].ProductID != cheese {
		t.Errorf("первым должен идти самый дорогой товар")
	}
	for _, it := range items {
		if it.ProductID == milk {
			if want := domain.MustParseMoney("178.00"); it.Spend != want {
				t.Errorf("расходы на молоко %s, ожидалось %s", it.Spend, want)
			}
			if it.Purchases != 2 {
				t.Errorf("покупок молока %d, ожидалось 2", it.Purchases)
			}
		}
	}
}

func TestReceiptsInPeriodAndLines(t *testing.T) {
	ctx := context.Background()
	budget := newBudget(t)
	stamp := time.Now().Format("150405.000")
	product := seedProduct(t, "хлеб тест "+stamp, "bakery")

	seedPurchase(t, budget, product, time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC), "45.00", "112.50")
	seedPurchase(t, budget, product, time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC), "49.00", "122.50")

	repo := NewAnalytics(pool)

	total, err := repo.CountReceipts(ctx, budget, testPeriod())
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("чеков %d, ожидалось 2", total)
	}

	receipts, err := repo.ReceiptsInPeriod(ctx, budget, testPeriod(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 2 {
		t.Fatalf("получено %d чеков", len(receipts))
	}
	// Свежие идут первыми.
	if !receipts[0].At.After(receipts[1].At) {
		t.Error("чеки должны идти от свежих к старым")
	}
	if receipts[0].Items != 1 {
		t.Errorf("позиций в чеке %d, ожидалась 1", receipts[0].Items)
	}

	head, lines, err := repo.ReceiptLines(ctx, budget, receipts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if head.Total != receipts[0].Total {
		t.Errorf("сумма чека %s, ожидалась %s", head.Total, receipts[0].Total)
	}
	if len(lines) != 1 {
		t.Fatalf("строк %d, ожидалась 1", len(lines))
	}
	if lines[0].Category == "" {
		t.Error("в строке чека должна быть категория")
	}
}

// Листание не должно повторять уже показанные чеки.
func TestReceiptsInPeriodPaging(t *testing.T) {
	ctx := context.Background()
	budget := newBudget(t)
	product := seedProduct(t, "вода тест "+time.Now().Format("150405.000"), "drinks")

	for i := 0; i < 5; i++ {
		at := time.Date(2026, 9, 10+i, 12, 0, 0, 0, time.UTC)
		seedPurchase(t, budget, product, at, "50.00", "50.00")
	}

	repo := NewAnalytics(pool)
	first, err := repo.ReceiptsInPeriod(ctx, budget, testPeriod(), 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.ReceiptsInPeriod(ctx, budget, testPeriod(), 2, 2)
	if err != nil {
		t.Fatal(err)
	}

	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("страницы: %d и %d", len(first), len(second))
	}
	for _, a := range first {
		for _, b := range second {
			if a.ID == b.ID {
				t.Errorf("чек %d попал на обе страницы", a.ID)
			}
		}
	}
}

// Чужой чек по прямой ссылке отдавать нельзя.
func TestReceiptLinesRejectsForeignBudget(t *testing.T) {
	ctx := context.Background()
	owner := newBudget(t)
	stranger := newBudget(t)
	product := seedProduct(t, "сок тест "+time.Now().Format("150405.000"), "drinks")

	seedPurchase(t, owner, product, time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC), "99.00", "99.00")

	repo := NewAnalytics(pool)
	receipts, err := repo.ReceiptsInPeriod(ctx, owner, testPeriod(), 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) == 0 {
		t.Fatal("чек не создан")
	}

	if _, _, err := repo.ReceiptLines(ctx, stranger, receipts[0].ID); err == nil {
		t.Error("чек чужого бюджета не должен отдаваться")
	}
}
