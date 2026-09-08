// Package qr разбирает фискальные QR-коды кассовых чеков вида
// t=20260906T1215&s=1543.20&fn=9960440300123456&i=12345&fp=1234567890&n=1
package qr

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

// ErrNotReceipt означает, что в кадр попал посторонний QR-код, а не чек.
var ErrNotReceipt = errors.New("строка не похожа на фискальный QR-код чека")

// Parse разбирает строку фискального QR-кода.
func Parse(raw string, loc *time.Location) (domain.Receipt, error) {
	query := strings.TrimSpace(raw)
	if query == "" {
		return domain.Receipt{}, fmt.Errorf("%w: пустая строка", ErrNotReceipt)
	}
	// Часть касс кодирует не голую query-строку, а ссылку на проверку чека.
	if _, after, found := strings.Cut(query, "?"); found {
		query = after
	}

	v, err := url.ParseQuery(query)
	if err != nil {
		return domain.Receipt{}, fmt.Errorf("%w: %v", ErrNotReceipt, err)
	}

	fn, fd, fp := v.Get("fn"), v.Get("i"), v.Get("fp")
	for _, f := range []struct{ name, val string }{{"fn", fn}, {"i", fd}, {"fp", fp}} {
		if f.val == "" {
			return domain.Receipt{}, fmt.Errorf("%w: нет поля %q", ErrNotReceipt, f.name)
		}
		if !isDigits(f.val) {
			return domain.Receipt{}, fmt.Errorf("%w: поле %q не число: %q", ErrNotReceipt, f.name, f.val)
		}
	}

	at, err := parseTime(v.Get("t"), loc)
	if err != nil {
		return domain.Receipt{}, err
	}

	total, err := domain.ParseMoney(v.Get("s"))
	if err != nil {
		return domain.Receipt{}, fmt.Errorf("сумма чека: %w", err)
	}

	op := domain.OpIncome
	if s := v.Get("n"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return domain.Receipt{}, fmt.Errorf("%w: тип операции %q", ErrNotReceipt, s)
		}
		op = domain.Operation(n)
	}
	// Отрицательная сумма избавляет агрегации от частных случаев.
	if op.IsRefund() {
		total = -total
	}

	return domain.Receipt{
		FN: fn, FD: fd, FP: fp,
		At:        at,
		Total:     total,
		Operation: op,
		Raw:       raw,
	}, nil
}

// Оба формата встречаются на кассах.
var timeLayouts = []string{"20060102T150405", "20060102T1504"}

func parseTime(s string, loc *time.Location) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("%w: нет поля даты", ErrNotReceipt)
	}
	if loc == nil {
		loc = time.UTC
	}
	for _, layout := range timeLayouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("дата чека: неизвестный формат %q", s)
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}
