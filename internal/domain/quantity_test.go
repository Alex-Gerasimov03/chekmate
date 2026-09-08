package domain

import (
	"errors"
	"testing"
)

func TestParseQuantity(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Quantity
	}{
		{"миллилитры без пробела", "930мл", Millilitres(930)},
		{"литры с запятой", "1,5л", Millilitres(1500)},
		{"литры с точкой и пробелом", "1.5 л", Millilitres(1500)},
		{"граммы", "500 г", Grams(500)},
		{"граммы сокращённо", "500гр", Grams(500)},
		{"килограммы дробные", "0,5кг", Grams(500)},
		{"килограммы целые", "2 кг", Grams(2000)},
		{"вес с точностью до грамма", "1,234кг", Grams(1234)},
		{"штуки", "2шт", Pieces(2)},
		{"число без единицы — штуки", "3", Pieces(3)},
		{"латинские единицы", "250 g", Grams(250)},
		{"верхний регистр", "930МЛ", Millilitres(930)},
		{"дробь без целой части", ",5кг", Grams(500)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseQuantity(tt.in)
			if err != nil {
				t.Fatalf("ParseQuantity(%q) вернул ошибку: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseQuantity(%q) = %+v, ожидалось %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseQuantityErrors(t *testing.T) {
	for _, in := range []string{"", "молоко", "кг", "1,5 попугая", "-5кг", "1..5л"} {
		t.Run(in, func(t *testing.T) {
			if got, err := ParseQuantity(in); err == nil {
				t.Fatalf("ParseQuantity(%q) = %+v, ожидалась ошибка", in, got)
			} else if !errors.Is(err, ErrBadQuantity) {
				t.Errorf("ошибка не оборачивает ErrBadQuantity: %v", err)
			}
		})
	}
}

func TestQuantityString(t *testing.T) {
	tests := []struct {
		in   Quantity
		want string
	}{
		{Millilitres(930), "930 мл"},
		{Millilitres(1500), "1,5 л"},
		{Millilitres(2000), "2 л"},
		{Grams(500), "500 г"},
		{Grams(1234), "1,234 кг"},
		{Grams(2000), "2 кг"},
		{Pieces(2), "2 шт"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.in.String(); got != tt.want {
				t.Errorf("String() = %q, ожидалось %q", got, tt.want)
			}
		})
	}
}

// Приведение к базовой единице — то, ради чего Quantity существует:
// одинаковый по сути объём должен совпадать независимо от записи в чеке.
func TestQuantityNormalisesToBaseUnit(t *testing.T) {
	pairs := [][2]string{
		{"1000г", "1кг"},
		{"1000мл", "1л"},
		{"0,25кг", "250г"},
		{"1,5л", "1500мл"},
	}
	for _, p := range pairs {
		if a, b := MustParseQuantity(p[0]), MustParseQuantity(p[1]); a != b {
			t.Errorf("%q и %q должны быть равны, получено %+v и %+v", p[0], p[1], a, b)
		}
	}
}

func TestQuantityScale(t *testing.T) {
	tests := []struct {
		name  string
		in    Quantity
		times float64
		want  Quantity
	}{
		{"две упаковки", Millilitres(930), 2, Millilitres(1860)},
		{"полторы", Grams(400), 1.5, Grams(600)},
		{"одна", Grams(400), 1, Grams(400)},
		{"дробный вес", Grams(1000), 0.756, Grams(756)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Scale(tt.times); got != tt.want {
				t.Errorf("Scale(%v) = %v, ожидалось %v", tt.times, got, tt.want)
			}
		})
	}
}
