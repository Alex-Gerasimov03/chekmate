package qr

import (
	"errors"
	"testing"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

var msk = time.FixedZone("MSK", 3*60*60)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantTotal domain.Money
		wantAt    time.Time
		wantOp    domain.Operation
	}{
		{
			name:      "обычная покупка",
			raw:       "t=20260906T1215&s=1543.20&fn=9960440300123456&i=12345&fp=1234567890&n=1",
			wantTotal: 154320,
			wantAt:    time.Date(2026, 9, 6, 12, 15, 0, 0, msk),
			wantOp:    domain.OpIncome,
		},
		{
			name:      "время с секундами",
			raw:       "t=20260906T121530&s=99.90&fn=9960440300123456&i=1&fp=2&n=1",
			wantTotal: 9990,
			wantAt:    time.Date(2026, 9, 6, 12, 15, 30, 0, msk),
			wantOp:    domain.OpIncome,
		},
		{
			name:      "возврат прихода уходит в минус",
			raw:       "t=20260906T1215&s=250.00&fn=9960440300123456&i=1&fp=2&n=2",
			wantTotal: -25000,
			wantAt:    time.Date(2026, 9, 6, 12, 15, 0, 0, msk),
			wantOp:    domain.OpIncomeRefund,
		},
		{
			name:      "сумма без копеек",
			raw:       "t=20260906T1215&s=500&fn=9960440300123456&i=1&fp=2&n=1",
			wantTotal: 50000,
			wantAt:    time.Date(2026, 9, 6, 12, 15, 0, 0, msk),
			wantOp:    domain.OpIncome,
		},
		{
			name:      "поля в произвольном порядке и с лишними",
			raw:       "n=1&fp=1234567890&s=10.00&extra=xyz&i=12345&t=20260906T1215&fn=9960440300123456",
			wantTotal: 1000,
			wantAt:    time.Date(2026, 9, 6, 12, 15, 0, 0, msk),
			wantOp:    domain.OpIncome,
		},
		{
			name:      "строка приехала целой ссылкой",
			raw:       "https://check.example/c?t=20260906T1215&s=10.00&fn=9960440300123456&i=1&fp=2&n=1",
			wantTotal: 1000,
			wantAt:    time.Date(2026, 9, 6, 12, 15, 0, 0, msk),
			wantOp:    domain.OpIncome,
		},
		{
			name:      "без поля n считаем покупкой",
			raw:       "t=20260906T1215&s=10.00&fn=9960440300123456&i=1&fp=2",
			wantTotal: 1000,
			wantAt:    time.Date(2026, 9, 6, 12, 15, 0, 0, msk),
			wantOp:    domain.OpIncome,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.raw, msk)
			if err != nil {
				t.Fatalf("Parse вернул ошибку: %v", err)
			}
			if got.Total != tt.wantTotal {
				t.Errorf("Total = %s, ожидалось %s", got.Total, tt.wantTotal)
			}
			if !got.At.Equal(tt.wantAt) {
				t.Errorf("At = %s, ожидалось %s", got.At, tt.wantAt)
			}
			if got.Operation != tt.wantOp {
				t.Errorf("Operation = %d, ожидалось %d", got.Operation, tt.wantOp)
			}
			if !got.IsFiscal() {
				t.Error("разобранный чек должен быть фискальным")
			}
			if got.Raw != tt.raw {
				t.Error("исходная строка должна сохраняться в Raw")
			}
		})
	}
}

func TestParseNotReceipt(t *testing.T) {
	tests := []struct{ name, raw string }{
		{"пустая строка", ""},
		{"посторонний QR со ссылкой", "https://example.com/promo"},
		{"нет фискального накопителя", "t=20260906T1215&s=10.00&i=1&fp=2&n=1"},
		{"нет номера документа", "t=20260906T1215&s=10.00&fn=996&fp=2&n=1"},
		{"нет фискального признака", "t=20260906T1215&s=10.00&fn=996&i=1&n=1"},
		{"фискальное поле не число", "t=20260906T1215&s=10.00&fn=abc&i=1&fp=2&n=1"},
		{"текст вместо чека", "просто какой-то текст"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.raw, msk); !errors.Is(err, ErrNotReceipt) {
				t.Errorf("ожидалась ErrNotReceipt, получено %v", err)
			}
		})
	}
}

func TestParseBrokenFields(t *testing.T) {
	tests := []struct{ name, raw string }{
		{"нет даты", "s=10.00&fn=996&i=1&fp=2&n=1"},
		{"неизвестный формат даты", "t=06.09.2026&s=10.00&fn=996&i=1&fp=2&n=1"},
		{"нет суммы", "t=20260906T1215&fn=996&i=1&fp=2&n=1"},
		{"сумма не число", "t=20260906T1215&s=много&fn=996&i=1&fp=2&n=1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.raw, msk); err == nil {
				t.Error("ожидалась ошибка")
			}
		})
	}
}

// В чужой зоне покупка уедет на другие сутки и попадёт не в тот период.
func TestParseRespectsLocation(t *testing.T) {
	const raw = "t=20260906T0030&s=10.00&fn=996&i=1&fp=2&n=1"

	inMSK, err := Parse(raw, msk)
	if err != nil {
		t.Fatal(err)
	}
	inUTC, err := Parse(raw, time.UTC)
	if err != nil {
		t.Fatal(err)
	}

	if d := inUTC.At.Sub(inMSK.At); d != 3*time.Hour {
		t.Errorf("разница зон = %v, ожидалось 3h", d)
	}
	if day := inMSK.At.UTC().Day(); day != 5 {
		t.Errorf("полночь 6-го по Москве — это 5-е число по UTC, получено %d", day)
	}
}
