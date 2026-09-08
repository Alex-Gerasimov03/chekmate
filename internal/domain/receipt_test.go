package domain

import (
	"errors"
	"testing"
)

func TestUnitPrice(t *testing.T) {
	tests := []struct {
		name string
		sum  string
		qty  string
		want string
	}{
		{"пачка масла 180 г за 249 ₽", "249.00", "180г", "1383.33"},
		{"молоко 930 мл за 89 ₽", "89.00", "930мл", "95.69"},
		{"килограмм ровно", "120.00", "1кг", "120.00"},
		{"две штуки", "50.00", "2шт", "25.00"},
		{"весовой товар", "185.50", "1,25кг", "148.40"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnitPrice(MustParseMoney(tt.sum), MustParseQuantity(tt.qty))
			if err != nil {
				t.Fatalf("UnitPrice вернул ошибку: %v", err)
			}
			if want := MustParseMoney(tt.want); got != want {
				t.Errorf("UnitPrice(%s, %s) = %s, ожидалось %s", tt.sum, tt.qty, got, want)
			}
		})
	}
}

func TestUnitPriceZeroQuantity(t *testing.T) {
	if _, err := UnitPrice(MustParseMoney("10.00"), Quantity{}); !errors.Is(err, ErrZeroQuantity) {
		t.Errorf("ожидалась ErrZeroQuantity, получено %v", err)
	}
}

// Шринкфляция: пачка ужалась, цена та же — ₽/кг обязан вырасти.
func TestUnitPriceDetectsShrinkflation(t *testing.T) {
	before, err := UnitPrice(MustParseMoney("249.00"), MustParseQuantity("400г"))
	if err != nil {
		t.Fatal(err)
	}
	after, err := UnitPrice(MustParseMoney("249.00"), MustParseQuantity("380г"))
	if err != nil {
		t.Fatal(err)
	}
	if after <= before {
		t.Errorf("уменьшение упаковки должно повышать цену за кг: было %s, стало %s", before, after)
	}
}

func TestReceiptFiscal(t *testing.T) {
	fiscal := Receipt{FN: "9960440300123456", FD: "12345", FP: "1234567890"}
	if !fiscal.IsFiscal() {
		t.Error("чек с фискальными полями должен считаться фискальным")
	}
	if want := "9960440300123456/12345/1234567890"; fiscal.FiscalKey() != want {
		t.Errorf("FiscalKey() = %q, ожидалось %q", fiscal.FiscalKey(), want)
	}
	if (Receipt{}).IsFiscal() {
		t.Error("чек без фискальных полей не должен считаться фискальным")
	}
}

func TestReceiptItemsTotal(t *testing.T) {
	r := Receipt{Items: []Item{
		{Sum: MustParseMoney("412.00")},
		{Sum: MustParseMoney("530.50")},
		{Sum: MustParseMoney("201.00")},
	}}
	if want := MustParseMoney("1143.50"); r.ItemsTotal() != want {
		t.Errorf("ItemsTotal() = %s, ожидалось %s", r.ItemsTotal(), want)
	}
}
