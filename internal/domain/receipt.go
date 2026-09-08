package domain

import (
	"errors"
	"fmt"
	"time"
)

// Operation — тип фискальной операции, поле "n" в QR-коде.
type Operation int

const (
	OpIncome        Operation = 1
	OpIncomeRefund  Operation = 2
	OpExpense       Operation = 3
	OpExpenseRefund Operation = 4
)

func (o Operation) IsRefund() bool { return o == OpIncomeRefund }

// Receipt — кассовый чек. FN, FD и FP заполнены только у чеков из QR-кода,
// у трат, введённых вручную, они пусты.
type Receipt struct {
	FN string
	FD string
	FP string

	At        time.Time
	Total     Money
	Operation Operation
	Merchant  string
	Items     []Item

	Raw string
}

type Item struct {
	RawName   string
	ProductID int64
	Qty       Quantity
	Sum       Money
	UnitPrice Money
}

func (r Receipt) IsFiscal() bool { return r.FN != "" && r.FD != "" && r.FP != "" }

// FiscalKey — ключ дедупликации: один и тот же чек фотографируют дважды.
func (r Receipt) FiscalKey() string { return r.FN + "/" + r.FD + "/" + r.FP }

// ItemsTotal расходится с Total, когда позиции получены не полностью, —
// это сигнал, что чек стоит разобрать вручную.
func (r Receipt) ItemsTotal() Money {
	var sum Money
	for _, it := range r.Items {
		sum += it.Sum
	}
	return sum
}

var ErrZeroQuantity = errors.New("нулевое количество")

// UnitPrice — цена за базовую единицу. Вся аналитика цен строится на ней:
// рубли за упаковку несопоставимы между магазинами и во времени.
func UnitPrice(sum Money, q Quantity) (Money, error) {
	if q.IsZero() {
		return 0, fmt.Errorf("%w: цену за единицу посчитать нельзя", ErrZeroQuantity)
	}
	return Money(int64(sum) * 1000 / q.Milli()), nil
}
