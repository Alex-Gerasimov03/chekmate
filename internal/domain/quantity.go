package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Unit — базовая единица измерения. Сравнивать цены между магазинами
// и во времени можно только приведя товары к одной единице.
type Unit string

const (
	UnitKg  Unit = "kg"
	UnitL   Unit = "l"
	UnitPcs Unit = "pcs"
)

// Quantity — количество в тысячных долях базовой единицы: граммах,
// миллилитрах, тысячных штуки.
type Quantity struct {
	milli int64
	unit  Unit
}

var ErrBadQuantity = errors.New("некорректное количество")

func NewQuantity(milli int64, u Unit) Quantity { return Quantity{milli: milli, unit: u} }

func Grams(g int64) Quantity       { return Quantity{milli: g, unit: UnitKg} }
func Millilitres(m int64) Quantity { return Quantity{milli: m, unit: UnitL} }
func Pieces(n int64) Quantity      { return Quantity{milli: n * 1000, unit: UnitPcs} }

func (q Quantity) Milli() int64 { return q.milli }
func (q Quantity) Unit() Unit   { return q.unit }
func (q Quantity) IsZero() bool { return q.milli == 0 }

// mult — сколько тысячных долей базовой единицы в одной такой единице.
var unitMilli = map[string]struct {
	mult int64
	base Unit
}{
	"г": {1, UnitKg}, "гр": {1, UnitKg}, "g": {1, UnitKg}, "гк": {1, UnitKg},
	"кг": {1000, UnitKg}, "kg": {1000, UnitKg},
	"мл": {1, UnitL}, "ml": {1, UnitL},
	"л": {1000, UnitL}, "l": {1000, UnitL}, "литр": {1000, UnitL},
	"шт": {1000, UnitPcs}, "шт.": {1000, UnitPcs}, "штук": {1000, UnitPcs},
	"pcs": {1000, UnitPcs}, "уп": {1000, UnitPcs}, "": {1000, UnitPcs},
}

var quantityRe = regexp.MustCompile(`^([0-9]*[.,]?[0-9]+)\s*([а-яa-z.]*)$`)

// ParseQuantity разбирает "930мл", "1,5 л", "0,5кг", "2шт".
// Число без единицы считается штуками.
func ParseQuantity(s string) (Quantity, error) {
	t := strings.ToLower(strings.TrimSpace(s))
	t = strings.Map(func(r rune) rune {
		if r == ' ' || r == ' ' {
			return ' '
		}
		return r
	}, t)

	m := quantityRe.FindStringSubmatch(t)
	if m == nil {
		return Quantity{}, fmt.Errorf("%w: %q", ErrBadQuantity, s)
	}

	u, ok := unitMilli[m[2]]
	if !ok {
		return Quantity{}, fmt.Errorf("%w: неизвестная единица %q", ErrBadQuantity, m[2])
	}

	thousandths, err := parseThousandths(m[1])
	if err != nil {
		return Quantity{}, fmt.Errorf("%w: %q", ErrBadQuantity, s)
	}

	return Quantity{milli: thousandths * u.mult / 1000, unit: u.base}, nil
}

// MustParseQuantity паникует при ошибке. Для литералов в тестах.
func MustParseQuantity(s string) Quantity {
	q, err := ParseQuantity(s)
	if err != nil {
		panic(err)
	}
	return q
}

func parseThousandths(s string) (int64, error) {
	whole, frac, _ := strings.Cut(strings.Replace(s, ",", ".", 1), ".")
	if whole == "" {
		whole = "0"
	}
	if !isDigits(whole) || len(whole) > 12 {
		return 0, ErrBadQuantity
	}
	n, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, err
	}
	n *= 1000

	if frac != "" {
		if !isDigits(frac) {
			return 0, ErrBadQuantity
		}
		if len(frac) > 3 {
			frac = frac[:3]
		}
		for len(frac) < 3 {
			frac += "0"
		}
		f, err := strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, err
		}
		n += f
	}
	return n, nil
}

// String выбирает удобную для чтения единицу: "930 мл", "1,5 л", "500 г".
func (q Quantity) String() string {
	switch q.unit {
	case UnitKg:
		if q.milli < 1000 {
			return formatMilli(q.milli*1000) + " г"
		}
		return formatMilli(q.milli) + " кг"
	case UnitL:
		if q.milli < 1000 {
			return formatMilli(q.milli*1000) + " мл"
		}
		return formatMilli(q.milli) + " л"
	case UnitPcs:
		return formatMilli(q.milli) + " шт"
	default:
		return formatMilli(q.milli)
	}
}

func formatMilli(milli int64) string {
	sign := ""
	if milli < 0 {
		sign, milli = "-", -milli
	}
	whole, frac := milli/1000, milli%1000
	if frac == 0 {
		return sign + strconv.FormatInt(whole, 10)
	}
	return sign + strconv.FormatInt(whole, 10) + "," + strings.TrimRight(fmt.Sprintf("%03d", frac), "0")
}

// Scale умножает количество: в чеке цена указана за упаковку, а куплено
// может быть несколько.
func (q Quantity) Scale(times float64) Quantity {
	if times <= 0 {
		return q
	}
	return Quantity{milli: int64(float64(q.milli)*times + 0.5), unit: q.unit}
}
