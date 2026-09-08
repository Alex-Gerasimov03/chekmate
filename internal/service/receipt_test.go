package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Alex-Gerasimov03/chekmate/internal/catalog"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
	"github.com/Alex-Gerasimov03/chekmate/internal/enrich"
	"github.com/Alex-Gerasimov03/chekmate/internal/qr"
)

const rawQR = "t=20260906T1215&s=1543.20&fn=9960440300123456&i=12345&fp=1234567890&n=1"

func TestAddFromQR(t *testing.T) {
	e := newEnv()

	added, err := e.svc.AddFromQR(context.Background(), 100, 7, rawQR)
	if err != nil {
		t.Fatal(err)
	}
	if !added.Created {
		t.Error("первый чек должен считаться новым")
	}
	if want := domain.MustParseMoney("1543.20"); added.Receipt.Total != want {
		t.Errorf("сумма %s, ожидалась %s", added.Receipt.Total, want)
	}
	if len(e.receipts.saved) != 1 || e.receipts.saved[0].source != SourceQR {
		t.Errorf("чек сохранён неверно: %+v", e.receipts.saved)
	}
}

// Повторно присланный чек не должен удваивать траты.
func TestAddFromQRDeduplicates(t *testing.T) {
	e := newEnv()
	ctx := context.Background()

	if _, err := e.svc.AddFromQR(ctx, 100, 7, rawQR); err != nil {
		t.Fatal(err)
	}
	added, err := e.svc.AddFromQR(ctx, 100, 7, rawQR)
	if err != nil {
		t.Fatal(err)
	}
	if added.Created {
		t.Error("повторный чек не должен создаваться заново")
	}
	if len(e.receipts.saved) != 1 {
		t.Errorf("сохранено чеков: %d, ожидался 1", len(e.receipts.saved))
	}
}

func TestAddFromQRRejectsForeignCode(t *testing.T) {
	if _, err := newEnv().svc.AddFromQR(context.Background(), 100, 7, "https://example.com"); !errors.Is(err, qr.ErrNotReceipt) {
		t.Errorf("ожидалась ErrNotReceipt, получено %v", err)
	}
}

func TestAddManualComputesUnitPrice(t *testing.T) {
	e := newEnv()

	added, err := e.svc.AddManual(context.Background(), 100, 7, "молоко простоквашино 930мл 89")
	if err != nil {
		t.Fatal(err)
	}
	if len(added.Receipt.Items) != 1 {
		t.Fatalf("позиций %d, ожидалась 1", len(added.Receipt.Items))
	}

	item := added.Receipt.Items[0]
	if want := domain.MustParseMoney("95.69"); item.UnitPrice != want {
		t.Errorf("цена за литр %s, ожидалась %s", item.UnitPrice, want)
	}
	if item.ProductID == 0 {
		t.Error("позиция должна быть привязана к товару")
	}
	if e.products.categories[item.ProductID] != dairy {
		t.Errorf("категория %d, ожидалась %d", e.products.categories[item.ProductID], dairy)
	}
}

func TestAddManualRejectsGarbage(t *testing.T) {
	if _, err := newEnv().svc.AddManual(context.Background(), 100, 7, "просто болтовня"); !errors.Is(err, ErrNotEntry) {
		t.Errorf("ожидалась ErrNotEntry, получено %v", err)
	}
}

// Пограничное совпадение не должно терять деньги: позиция сохраняется,
// уточняется только принадлежность товару.
func TestAmbiguousItemKeepsMoney(t *testing.T) {
	e := newEnv()
	e.catalog.similar = []catalog.Candidate{{ProductID: 42, Title: "молоко деревенское", Similarity: 0.45}}

	added, err := e.svc.AddManual(context.Background(), 100, 7, "молоко фермерское 89")
	if err != nil {
		t.Fatal(err)
	}

	if len(added.Questions) != 1 {
		t.Fatalf("вопросов %d, ожидался 1", len(added.Questions))
	}
	if added.Questions[0].ReceiptID != added.ReceiptID {
		t.Error("вопрос должен ссылаться на сохранённый чек")
	}
	if len(added.Receipt.Items) != 1 {
		t.Fatal("позиция должна сохраниться несмотря на неопределённость")
	}
	if added.Receipt.Items[0].ProductID != 0 {
		t.Error("товар не должен быть привязан до ответа пользователя")
	}
	if want := domain.MustParseMoney("89.00"); added.Receipt.Total != want {
		t.Errorf("сумма %s, ожидалась %s", added.Receipt.Total, want)
	}
}

func TestConfirmProduct(t *testing.T) {
	e := newEnv()
	ctx := context.Background()

	if err := e.svc.ConfirmProduct(ctx, 1, "молоко фермерское", 42); err != nil {
		t.Fatal(err)
	}
	if e.receipts.assigned["молоко фермерское"] != 42 {
		t.Error("позиция должна быть привязана к товару")
	}
	if e.catalog.aliases["молоко фермерское"] != 42 {
		t.Error("ответ пользователя должен запомниться алиасом")
	}
}

func TestRejectSuggestionCreatesOwnProduct(t *testing.T) {
	e := newEnv()

	if err := e.svc.RejectSuggestion(context.Background(), 1, 1, "молоко фермерское"); err != nil {
		t.Fatal(err)
	}

	productID := e.receipts.assigned["молоко фермерское"]
	if productID == 0 {
		t.Fatal("позиция должна быть привязана к новому товару")
	}
	if e.products.categories[productID] != dairy {
		t.Errorf("новому товару должна назначаться категория, получено %d", e.products.categories[productID])
	}
}

// Новое правило обязано подействовать и на уже купленные товары, иначе
// в отчёте всё осталось бы по-старому.
func TestAddRuleRecategorizesExisting(t *testing.T) {
	e := newEnv()
	e.products.byBudget = []catalog.ProductRef{{ID: 5, Canonical: "сгущенк молочн"}}

	changed, err := e.svc.AddRule(context.Background(), 1, grocery, "сгущенка")
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Errorf("пересчитано товаров: %d, ожидался 1", changed)
	}
	if e.products.categories[5] != grocery {
		t.Errorf("категория %d, ожидалась %d", e.products.categories[5], grocery)
	}
}

// fakeEnricher отдаёт состав чека, как это делал бы внешний сервис.
type fakeEnricher struct {
	result enrich.Result
	err    error
}

func (f fakeEnricher) Enrich(context.Context, domain.Receipt) (enrich.Result, error) {
	return f.result, f.err
}

func TestAddFromQRUsesReceiptItems(t *testing.T) {
	e := newEnv()
	e.svc.enricher = fakeEnricher{result: enrich.Result{
		Merchant: "Пятёрочка",
		Items: []enrich.RawItem{
			{Name: "МОЛОКО ПРОСТОКВ.3,2% 930МЛ", Count: 1, Sum: domain.MustParseMoney("89.00")},
			{Name: "БАНАНЫ", Count: 0.831, Sum: domain.MustParseMoney("99.72")},
		},
	}}

	added, err := e.svc.AddFromQR(context.Background(), 100, 7, rawQR)
	if err != nil {
		t.Fatal(err)
	}

	if added.Receipt.Merchant != "Пятёрочка" {
		t.Errorf("магазин %q", added.Receipt.Merchant)
	}
	if len(added.Receipt.Items) != 2 {
		t.Fatalf("позиций %d, ожидалось 2", len(added.Receipt.Items))
	}

	// Объём берётся из названия, а не из количества упаковок.
	milk := added.Receipt.Items[0]
	if want := domain.Millilitres(930); milk.Qty != want {
		t.Errorf("количество %v, ожидалось %v", milk.Qty, want)
	}
	if want := domain.MustParseMoney("95.69"); milk.UnitPrice != want {
		t.Errorf("цена за литр %s, ожидалась %s", milk.UnitPrice, want)
	}

	// У весового товара объёма в названии нет: количество идёт как есть.
	if bananas := added.Receipt.Items[1]; bananas.Qty.Milli() != 831 {
		t.Errorf("количество %v, ожидалось 0,831", bananas.Qty)
	}
}

// Недоступный источник состава не должен ронять добавление чека.
func TestAddFromQRSurvivesEnricherFailure(t *testing.T) {
	e := newEnv()
	e.svc.enricher = fakeEnricher{err: errors.New("сервис недоступен")}

	added, err := e.svc.AddFromQR(context.Background(), 100, 7, rawQR)
	if err != nil {
		t.Fatal(err)
	}
	if !added.Created {
		t.Error("чек должен сохраняться и без состава")
	}
	if want := domain.MustParseMoney("1543.20"); added.Receipt.Total != want {
		t.Errorf("сумма %s, ожидалась %s", added.Receipt.Total, want)
	}
	if len(added.Receipt.Items) != 0 {
		t.Errorf("позиций быть не должно: %+v", added.Receipt.Items)
	}
}

// Несколько упаковок одного товара: объём умножается на количество.
func TestItemsFromScalesPackQuantity(t *testing.T) {
	e := newEnv()

	items := e.svc.itemsFrom([]enrich.RawItem{
		{Name: "МОЛОКО 930МЛ", Count: 2, Sum: domain.MustParseMoney("178.00")},
	})

	if len(items) != 1 {
		t.Fatalf("позиций %d", len(items))
	}
	if want := domain.Millilitres(1860); items[0].Qty != want {
		t.Errorf("количество %v, ожидалось %v", items[0].Qty, want)
	}
	if want := domain.MustParseMoney("95.69"); items[0].UnitPrice != want {
		t.Errorf("цена за литр %s, ожидалась %s", items[0].UnitPrice, want)
	}
}
