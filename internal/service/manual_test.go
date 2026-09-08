package service

import (
	"errors"
	"testing"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

func TestParseManual(t *testing.T) {
	tests := []struct {
		in       string
		wantName string
		wantQty  domain.Quantity
		wantSum  string
	}{
		{"кофе 300", "кофе", domain.Pieces(1), "300.00"},
		{"молоко простоквашино 930мл 89", "молоко простоквашино", domain.Millilitres(930), "89.00"},
		{"масло 180г 249.90", "масло", domain.Grams(180), "249.90"},
		{"картофель 2кг 120", "картофель", domain.Grams(2000), "120.00"},
		{"хлеб 45,50", "хлеб", domain.Pieces(1), "45.50"},
		{"  сыр   российский   399  ", "сыр российский", domain.Pieces(1), "399.00"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseManual(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if got.Name != tt.wantName {
				t.Errorf("название %q, ожидалось %q", got.Name, tt.wantName)
			}
			if got.Qty != tt.wantQty {
				t.Errorf("количество %v, ожидалось %v", got.Qty, tt.wantQty)
			}
			if want := domain.MustParseMoney(tt.wantSum); got.Sum != want {
				t.Errorf("сумма %s, ожидалась %s", got.Sum, want)
			}
		})
	}
}

// Название из одного слова не должно съедаться разбором количества.
func TestParseManualKeepsSingleWordName(t *testing.T) {
	got, err := ParseManual("хлеб 45")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "хлеб" {
		t.Errorf("название %q, ожидалось \"хлеб\"", got.Name)
	}
}

func TestParseManualErrors(t *testing.T) {
	for _, in := range []string{"", "молоко", "300", "молоко ноль", "молоко -50", "молоко 0"} {
		t.Run(in, func(t *testing.T) {
			if got, err := ParseManual(in); err == nil {
				t.Errorf("ParseManual(%q) = %+v, ожидалась ошибка", in, got)
			} else if !errors.Is(err, ErrNotEntry) {
				t.Errorf("ошибка не оборачивает ErrNotEntry: %v", err)
			}
		})
	}
}

// Магазина нет в QR-коде, поэтому при ручном вводе его можно указать явно.
func TestParseManualMerchant(t *testing.T) {
	got, err := ParseManual("молоко 930мл 89 @Лента")
	if err != nil {
		t.Fatal(err)
	}
	if got.Merchant != "Лента" {
		t.Errorf("магазин %q, ожидалась \"Лента\"", got.Merchant)
	}
	if got.Name != "молоко" {
		t.Errorf("название %q, ожидалось \"молоко\"", got.Name)
	}
	if want := domain.MustParseMoney("89.00"); got.Sum != want {
		t.Errorf("сумма %s, ожидалась %s", got.Sum, want)
	}
}

func TestParseManualMerchantAnywhere(t *testing.T) {
	got, err := ParseManual("@Пятёрочка хлеб 45")
	if err != nil {
		t.Fatal(err)
	}
	if got.Merchant != "Пятёрочка" || got.Name != "хлеб" {
		t.Errorf("разобрано неверно: %+v", got)
	}
}
