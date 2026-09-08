// Package domain содержит типы предметной области.
package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Money — сумма в копейках. Целые числа выбраны, чтобы итог сходился
// с чеком: float накапливает ошибку уже на сотне позиций.
type Money int64

var ErrBadMoney = errors.New("некорректная денежная сумма")

const kopecksInRuble = 100

// ParseMoney разбирает сумму в рублях: "1543.20", "1 543,20", "-99", "0.05".
func ParseMoney(s string) (Money, error) {
	t := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	t = strings.Replace(t, ",", ".", 1)

	if t == "" {
		return 0, fmt.Errorf("%w: пустая строка", ErrBadMoney)
	}

	neg := false
	switch t[0] {
	case '-':
		neg, t = true, t[1:]
	case '+':
		t = t[1:]
	}

	whole, frac, hasFrac := strings.Cut(t, ".")
	if whole == "" {
		whole = "0"
	}
	if !isDigits(whole) || len(whole) > 15 {
		return 0, fmt.Errorf("%w: %q", ErrBadMoney, s)
	}

	rubles, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrBadMoney, s)
	}

	var kopecks int64
	if hasFrac {
		if !isDigits(frac) || len(frac) > 2 {
			return 0, fmt.Errorf("%w: ожидались копейки в %q", ErrBadMoney, s)
		}
		if len(frac) == 1 {
			frac += "0" // "1543.2" — это 20 копеек, а не 2
		}
		if kopecks, err = strconv.ParseInt(frac, 10, 64); err != nil {
			return 0, fmt.Errorf("%w: %q", ErrBadMoney, s)
		}
	}

	m := Money(rubles*kopecksInRuble + kopecks)
	if neg {
		m = -m
	}
	return m, nil
}

// MustParseMoney паникует при ошибке. Для литералов в тестах.
func MustParseMoney(s string) Money {
	m, err := ParseMoney(s)
	if err != nil {
		panic(err)
	}
	return m
}

func (m Money) String() string {
	sign, v := "", int64(m)
	if v < 0 {
		sign, v = "-", -v
	}
	return fmt.Sprintf("%s%s,%02d ₽", sign, groupThousands(v/kopecksInRuble), v%kopecksInRuble)
}

// Rubles нужен только для вывода: считать следует в Money.
func (m Money) Rubles() float64 { return float64(m) / kopecksInRuble }

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	head := len(s) % 3
	if head > 0 {
		b.WriteString(s[:head])
	}
	for i := head; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
