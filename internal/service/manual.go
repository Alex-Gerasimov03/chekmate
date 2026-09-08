// Package service связывает разбор чека, справочник товаров и хранилище.
package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

// ManualEntry — трата, введённая текстом: "молоко 930мл 89 @Лента".
type ManualEntry struct {
	Name     string
	Qty      domain.Quantity
	Sum      domain.Money
	Merchant string
}

var ErrNotEntry = errors.New("не похоже на запись о трате")

// ParseManual разбирает строку вида "название [количество] сумма [@магазин]".
func ParseManual(text string) (ManualEntry, error) {
	fields := strings.Fields(strings.TrimSpace(text))

	var merchant string
	kept := fields[:0]
	for _, f := range fields {
		if strings.HasPrefix(f, "@") && len(f) > 1 {
			merchant = f[1:]
			continue
		}
		kept = append(kept, f)
	}
	fields = kept
	if len(fields) < 2 {
		return ManualEntry{}, fmt.Errorf("%w: нужно название и сумма", ErrNotEntry)
	}

	sum, err := domain.ParseMoney(fields[len(fields)-1])
	if err != nil {
		return ManualEntry{}, fmt.Errorf("%w: последним должно идти число", ErrNotEntry)
	}
	if sum <= 0 {
		return ManualEntry{}, fmt.Errorf("%w: сумма должна быть положительной", ErrNotEntry)
	}
	fields = fields[:len(fields)-1]

	entry := ManualEntry{Sum: sum, Qty: domain.Pieces(1), Merchant: merchant}
	if len(fields) > 1 {
		if q, err := domain.ParseQuantity(fields[len(fields)-1]); err == nil && !q.IsZero() {
			entry.Qty = q
			fields = fields[:len(fields)-1]
		}
	}

	entry.Name = strings.Join(fields, " ")
	if entry.Name == "" {
		return ManualEntry{}, fmt.Errorf("%w: не указано название", ErrNotEntry)
	}
	return entry, nil
}
