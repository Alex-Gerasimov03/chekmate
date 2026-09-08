package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

func benchReceipt(n int) domain.Receipt {
	items := make([]domain.Item, 12)
	for i := range items {
		items[i] = domain.Item{
			RawName:   fmt.Sprintf("ТОВАР %d", i),
			Qty:       domain.Grams(500),
			Sum:       domain.MustParseMoney("100.00"),
			UnitPrice: domain.MustParseMoney("200.00"),
		}
	}
	return domain.Receipt{
		FN: "996044030012", FD: fmt.Sprintf("%d", n), FP: "1234567890",
		At:    time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC),
		Total: domain.MustParseMoney("1200.00"),
		Items: items,
	}
}

// Сохранение чека с составом — основная операция при добавлении.
func BenchmarkSaveReceipt(b *testing.B) {
	ctx := context.Background()
	repo := NewReceipts(pool)
	budget := benchBudget(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := repo.Save(ctx, budget, 1, SourceQR, benchReceipt(i)); err != nil {
			b.Fatal(err)
		}
	}
}

// Отчёт по категориям — самый частый запрос из бота.
func BenchmarkCategorySpends(b *testing.B) {
	ctx := context.Background()
	budget := benchBudget(b)
	seedBench(b, budget, 200)

	cur := analytics.Period{
		From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	prev := analytics.Period{From: cur.From.AddDate(0, -1, 0), To: cur.From}

	repo := NewAnalytics(pool)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.CategorySpends(ctx, budget, cur, prev); err != nil {
			b.Fatal(err)
		}
	}
}

// Наблюдения цен считают медиану по всем позициям периода.
func BenchmarkPriceObservations(b *testing.B) {
	ctx := context.Background()
	budget := benchBudget(b)
	seedBench(b, budget, 200)

	period := analytics.Period{
		From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}

	repo := NewAnalytics(pool)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.PriceObservations(ctx, budget, period); err != nil {
			b.Fatal(err)
		}
	}
}

// Отчёт под одновременными запросами: так ведёт себя бот при нескольких чатах.
func BenchmarkCategorySpendsParallel(b *testing.B) {
	ctx := context.Background()
	budget := benchBudget(b)
	seedBench(b, budget, 200)

	cur := analytics.Period{
		From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	prev := analytics.Period{From: cur.From.AddDate(0, -1, 0), To: cur.From}
	repo := NewAnalytics(pool)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := repo.CategorySpends(ctx, budget, cur, prev); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func benchBudget(b *testing.B) int64 {
	b.Helper()
	id, err := NewBudgets(pool).GetOrCreate(context.Background(), time.Now().UnixNano(), "Europe/Moscow")
	if err != nil {
		b.Fatal(err)
	}
	return id
}

// seedBench наполняет бюджет чеками, чтобы запросы работали не на пустой базе.
func seedBench(b *testing.B, budget int64, receipts int) {
	b.Helper()
	ctx := context.Background()
	repo := NewReceipts(pool)
	stamp := time.Now().UnixNano()

	for i := 0; i < receipts; i++ {
		r := benchReceipt(i)
		r.FD = fmt.Sprintf("%d-%d", stamp, i)
		if _, _, err := repo.Save(ctx, budget, 1, SourceQR, r); err != nil {
			b.Fatal(err)
		}
	}
}
