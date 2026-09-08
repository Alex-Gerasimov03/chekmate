package domain

import (
	"errors"
	"testing"
)

func TestParseMoney(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Money
	}{
		{"рубли с копейками", "1543.20", 154320},
		{"запятая как разделитель", "1543,20", 154320},
		{"пробелы-разделители разрядов", "1 543,20", 154320},
		{"неразрывный пробел", "1 543,20", 154320},
		{"без дробной части", "1543", 154300},
		{"один знак после точки — это десятки копеек", "1543.2", 154320},
		{"только копейки", "0.05", 5},
		{"ноль", "0", 0},
		{"возврат — отрицательная сумма", "-99.90", -9990},
		{"явный плюс", "+10.00", 1000},
		{"без целой части", ".50", 50},
		{"обрамляющие пробелы", "  12.34  ", 1234},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMoney(tt.in)
			if err != nil {
				t.Fatalf("ParseMoney(%q) вернул ошибку: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseMoney(%q) = %d, ожидалось %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseMoneyErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"пустая строка", ""},
		{"только пробелы", "   "},
		{"не число", "abc"},
		{"мусор в дробной части", "12.3x"},
		{"больше двух знаков после точки", "12.345"},
		{"точка без копеек", "12."},
		{"два разделителя", "1.2.3"},
		{"знак внутри дробной части", "12.-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMoney(tt.in)
			if err == nil {
				t.Fatalf("ParseMoney(%q) = %d, ожидалась ошибка", tt.in, got)
			}
			if !errors.Is(err, ErrBadMoney) {
				t.Errorf("ошибка не оборачивает ErrBadMoney: %v", err)
			}
		})
	}
}

func TestMoneyString(t *testing.T) {
	tests := []struct {
		in   Money
		want string
	}{
		{154320, "1 543,20 ₽"},
		{5, "0,05 ₽"},
		{0, "0,00 ₽"},
		{100, "1,00 ₽"},
		{-9990, "-99,90 ₽"},
		{123456789, "1 234 567,89 ₽"},
		{99999, "999,99 ₽"},
		{100000, "1 000,00 ₽"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.in.String(); got != tt.want {
				t.Errorf("Money(%d).String() = %q, ожидалось %q", int64(tt.in), got, tt.want)
			}
		})
	}
}

// Ради этого свойства деньги и хранятся в целых копейках.
func TestMoneySumIsExact(t *testing.T) {
	var total Money
	for i := 0; i < 1000; i++ {
		total += MustParseMoney("0.10")
	}
	if want := MustParseMoney("100.00"); total != want {
		t.Errorf("сумма 1000 раз по 0,10 = %s, ожидалось %s", total, want)
	}
}
