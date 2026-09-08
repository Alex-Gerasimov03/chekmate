// Package enrich добывает состав чека по его фискальным реквизитам.
package enrich

import (
	"errors"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

// RawItem — позиция чека, как её отдал источник. Единица измерения здесь ещё
// не определена: она вытаскивается из названия на стороне сервиса.
type RawItem struct {
	Name  string
	Count float64
	Sum   domain.Money
}

// Result — то, что удалось узнать о чеке дополнительно к QR-коду.
type Result struct {
	Items    []RawItem
	Merchant string
}

// ErrNotFound означает, что чек существует, но источник его не отдал:
// данные ещё не дошли от кассы или чек слишком старый.
var ErrNotFound = errors.New("состав чека не найден")
