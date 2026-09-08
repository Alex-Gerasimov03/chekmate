package postgres

import (
	"context"
	"testing"

	"github.com/Alex-Gerasimov03/chekmate/internal/catalog"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

func milk() catalog.NewProduct {
	return catalog.NewProduct{
		Canonical: "молок простоквашин",
		Title:     "молоко простоквашино",
		Fat:       320,
		Unit:      domain.UnitL,
		PackMilli: 930,
		Version:   1,
	}
}

func TestProductsCreateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := NewProducts(pool)

	first, err := repo.CreateProduct(ctx, milk())
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateProduct(ctx, milk())
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("товар задвоился: %d и %d", first, second)
	}
}

// Кефир 1% и 3,2% — разные товары, и упаковки разного объёма тоже.
func TestProductsIdentityIncludesFatAndPack(t *testing.T) {
	ctx := context.Background()
	repo := NewProducts(pool)

	base := catalog.NewProduct{
		Canonical: "кефир", Title: "кефир", Fat: 100,
		Unit: domain.UnitL, PackMilli: 900, Version: 1,
	}
	light, err := repo.CreateProduct(ctx, base)
	if err != nil {
		t.Fatal(err)
	}

	base.Fat = 320
	fatty, err := repo.CreateProduct(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if light == fatty {
		t.Error("разная жирность должна давать разные товары")
	}

	base.PackMilli = 1000
	bigger, err := repo.CreateProduct(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if bigger == fatty {
		t.Error("разный объём упаковки должен давать разные товары")
	}
}

// Ради этого запроса в схеме и стоит триграммный индекс: написания в чеках
// разных сетей не совпадают буквально.
func TestProductsFindSimilar(t *testing.T) {
	ctx := context.Background()
	repo := NewProducts(pool)

	id, err := repo.CreateProduct(ctx, catalog.NewProduct{
		Canonical: "сыр российск голланд", Title: "сыр российский голландский",
		Fat: 5000, Unit: domain.UnitKg, PackMilli: 200, Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindSimilar(ctx, "сыр российск", domain.UnitKg, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("похожий товар не найден")
	}
	if got[0].ProductID != id {
		t.Errorf("найден товар %d, ожидался %d", got[0].ProductID, id)
	}
	if got[0].Similarity <= 0 || got[0].Similarity > 1 {
		t.Errorf("сходство вне диапазона: %f", got[0].Similarity)
	}

	other, err := repo.FindSimilar(ctx, "сыр российск", domain.UnitL, 5000)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range other {
		if c.ProductID == id {
			t.Error("товар другой единицы измерения не должен попадать в кандидаты")
		}
	}
}

// Подтверждение пользователя не должно теряться при повторной автопривязке.
func TestProductsSaveAliasKeepsConfirmation(t *testing.T) {
	ctx := context.Background()
	repo := NewProducts(pool)

	id, err := repo.CreateProduct(ctx, milk())
	if err != nil {
		t.Fatal(err)
	}
	const raw = "МОЛ.ПРОСТОКВ 930"

	if err := repo.SaveAlias(ctx, raw, id, true); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveAlias(ctx, raw, id, false); err != nil {
		t.Fatal(err)
	}

	var confirmed bool
	err = pool.QueryRow(ctx,
		`SELECT confirmed FROM product_aliases WHERE raw_name = $1`, raw).Scan(&confirmed)
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed {
		t.Error("подтверждение пользователя должно сохраняться")
	}

	gotID, found, err := repo.AliasProduct(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !found || gotID != id {
		t.Errorf("алиас вернул %d (found=%v), ожидался %d", gotID, found, id)
	}
}

func TestDictionariesLoad(t *testing.T) {
	ctx := context.Background()

	dict, err := NewDictionaries(pool).Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dict.Abbrev["мол"] != "молоко" {
		t.Errorf("сокращение не загрузилось: %q", dict.Abbrev["мол"])
	}
	if !dict.StopWords["шт"] {
		t.Error("стоп-слова не загрузились")
	}
	if dict.Version == 0 {
		t.Error("версия словаря должна быть ненулевой")
	}
}
