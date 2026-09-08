package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

func fiscalReceipt() domain.Receipt {
	return domain.Receipt{
		FN: "9960440300123456", FD: "12345", FP: "1234567890",
		At:        time.Date(2026, 9, 6, 12, 15, 0, 0, time.UTC),
		Total:     domain.MustParseMoney("1543.20"),
		Operation: domain.OpIncome,
		Merchant:  "Пятёрочка",
		Raw:       "t=20260906T1215&s=1543.20&fn=9960440300123456&i=12345&fp=1234567890&n=1",
		Items: []domain.Item{
			{RawName: "МОЛОКО 930МЛ", Qty: domain.Millilitres(930), Sum: domain.MustParseMoney("89.00"), UnitPrice: domain.MustParseMoney("95.69")},
			{RawName: "ХЛЕБ 400Г", Qty: domain.Grams(400), Sum: domain.MustParseMoney("45.50"), UnitPrice: domain.MustParseMoney("113.75")},
		},
	}
}

func TestBudgetsGetOrCreateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := NewBudgets(pool)
	chatID := time.Now().UnixNano()

	first, err := repo.GetOrCreate(ctx, chatID, "Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.GetOrCreate(ctx, chatID, "Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("повторный вызов создал новый бюджет: %d и %d", first, second)
	}
}

// Один и тот же чек фотографируют дважды — второй раз он не должен удваивать траты.
func TestReceiptsSaveDeduplicates(t *testing.T) {
	ctx := context.Background()
	repo := NewReceipts(pool)
	budget := newBudget(t)
	r := fiscalReceipt()

	id, created, err := repo.Save(ctx, budget, 1, SourceQR, r)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("первый чек должен считаться новым")
	}

	sameID, created, err := repo.Save(ctx, budget, 1, SourceQR, r)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Error("повторный чек не должен создаваться заново")
	}
	if sameID != id {
		t.Errorf("вернулся другой идентификатор: %d вместо %d", sameID, id)
	}

	if n, err := repo.CountByBudget(ctx, budget); err != nil {
		t.Fatal(err)
	} else if n != 1 {
		t.Errorf("в бюджете %d чеков, ожидался 1", n)
	}
}

func TestReceiptsSaveStoresItems(t *testing.T) {
	ctx := context.Background()
	budget := newBudget(t)
	r := fiscalReceipt()
	r.FD = "77777"

	id, _, err := NewReceipts(pool).Save(ctx, budget, 1, SourceQR, r)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	var sum int64
	err = pool.QueryRow(ctx,
		`SELECT count(*), coalesce(sum(sum), 0) FROM receipt_items WHERE receipt_id = $1`, id,
	).Scan(&count, &sum)
	if err != nil {
		t.Fatal(err)
	}
	if count != len(r.Items) {
		t.Errorf("сохранено %d позиций, ожидалось %d", count, len(r.Items))
	}
	if want := int64(r.ItemsTotal()); sum != want {
		t.Errorf("сумма позиций в базе %d, ожидалась %d", sum, want)
	}
}

// У ручных трат фискальных полей нет, и частичный индекс не должен мешать
// сохранять две одинаковые покупки: в магазин можно сходить дважды за день.
func TestReceiptsManualEntriesAreNotDeduplicated(t *testing.T) {
	ctx := context.Background()
	repo := NewReceipts(pool)
	budget := newBudget(t)

	manual := domain.Receipt{
		At:    time.Now().UTC(),
		Total: domain.MustParseMoney("300.00"),
	}

	for i := 0; i < 2; i++ {
		if _, created, err := repo.Save(ctx, budget, 1, SourceManual, manual); err != nil {
			t.Fatal(err)
		} else if !created {
			t.Fatalf("ручная трата №%d должна создаваться", i+1)
		}
	}

	if n, err := repo.CountByBudget(ctx, budget); err != nil {
		t.Fatal(err)
	} else if n != 2 {
		t.Errorf("в бюджете %d трат, ожидалось 2", n)
	}
}
