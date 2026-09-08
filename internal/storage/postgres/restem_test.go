package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/catalog"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
	"github.com/Alex-Gerasimov03/chekmate/internal/normalize"
)

func dictWith(abbrev map[string]string, version int) *normalize.Normalizer {
	return normalize.New(normalize.Dictionary{
		Abbrev:    abbrev,
		StopWords: map[string]bool{},
		Version:   version,
	})
}

// Пока словарь не знает сокращения, товар заводится под одним ключом;
// после пополнения словаря ключ обязан пересчитаться.
func TestRestemUpdatesCanonicalName(t *testing.T) {
	ctx := context.Background()
	repo := NewProducts(pool)
	raw := "МОЛ.ТЕСТ " + "m" + time.Now().Format("150405")

	old := dictWith(map[string]string{}, 1)
	res := old.Name(raw)

	id, err := repo.CreateProduct(ctx, catalog.NewProduct{
		Canonical: res.Canonical, Title: res.Title, Fat: res.Fat,
		Unit: domain.UnitPcs, PackMilli: res.Qty.Milli(), Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveAlias(ctx, raw, id, true); err != nil {
		t.Fatal(err)
	}

	updated, merged, err := repo.Restem(ctx, dictWith(map[string]string{"мол": "молоко"}, 2))
	if err != nil {
		t.Fatal(err)
	}
	if updated == 0 && merged == 0 {
		t.Fatal("товар не был пересчитан")
	}

	var canonical string
	var version int
	err = pool.QueryRow(ctx,
		`SELECT canonical_name, dict_version FROM products WHERE id = $1`, id).Scan(&canonical, &version)
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Errorf("версия словаря %d, ожидалась 2", version)
	}
	if canonical == res.Canonical {
		t.Errorf("ключ не изменился: %q", canonical)
	}
}

// Если после пересчёта товар совпал с уже существующим, записи должны
// слиться, а история покупок — сохраниться целиком.
func TestRestemMergesDuplicates(t *testing.T) {
	ctx := context.Background()
	repo := NewProducts(pool)
	budget := newBudget(t)
	stamp := "m" + time.Now().Format("150405")

	target := "молоко тест " + stamp
	norm := dictWith(map[string]string{"мол": "молоко"}, 2)
	res := norm.Name(target)

	// Товар, к которому всё должно сойтись.
	twin, err := repo.CreateProduct(ctx, catalog.NewProduct{
		Canonical: res.Canonical, Title: res.Title, Fat: res.Fat,
		Unit: domain.UnitPcs, PackMilli: res.Qty.Milli(), Version: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Двойник, заведённый до пополнения словаря.
	rawOld := "мол тест " + stamp
	old := dictWith(map[string]string{}, 1)
	oldRes := old.Name(rawOld)

	stale, err := repo.CreateProduct(ctx, catalog.NewProduct{
		Canonical: oldRes.Canonical, Title: oldRes.Title, Fat: oldRes.Fat,
		Unit: domain.UnitPcs, PackMilli: oldRes.Qty.Milli(), Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stale == twin {
		t.Fatal("для проверки нужны два разных товара")
	}
	if err := repo.SaveAlias(ctx, rawOld, stale, true); err != nil {
		t.Fatal(err)
	}
	seedPurchase(t, budget, stale, time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC), "100.00", "100.00")

	if _, merged, err := repo.Restem(ctx, norm); err != nil {
		t.Fatal(err)
	} else if merged == 0 {
		t.Fatal("дубликат не был слит")
	}

	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM products WHERE id = $1)`, stale).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("дубликат должен быть удалён")
	}

	var items int
	var sum int64
	err = pool.QueryRow(ctx,
		`SELECT count(*), coalesce(sum(sum), 0) FROM receipt_items WHERE product_id = $1`, twin).Scan(&items, &sum)
	if err != nil {
		t.Fatal(err)
	}
	if items == 0 {
		t.Fatal("покупки не перенесены на оставшийся товар")
	}
	if want := int64(domain.MustParseMoney("100.00")); sum != want {
		t.Errorf("сумма перенесённых покупок %d, ожидалась %d", sum, want)
	}

	var aliasTarget int64
	if err := pool.QueryRow(ctx, `SELECT product_id FROM product_aliases WHERE raw_name = $1`, rawOld).Scan(&aliasTarget); err != nil {
		t.Fatal(err)
	}
	if aliasTarget != twin {
		t.Errorf("алиас указывает на %d, ожидался %d", aliasTarget, twin)
	}
}
