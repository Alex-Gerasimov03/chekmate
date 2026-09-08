package enrich

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

func fiscalReceipt() domain.Receipt {
	return domain.Receipt{
		FN: "9960440300123456", FD: "12345", FP: "1234567890",
		At:        time.Date(2026, 9, 6, 12, 15, 0, 0, time.UTC),
		Total:     domain.MustParseMoney("154.32"),
		Operation: domain.OpIncome,
	}
}

// serveFile поднимает подставной сервис, отдающий заготовленный ответ.
func serveFile(t *testing.T, path string, check func(*http.Request)) *httptest.Server {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			check(r)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func TestEnrichParsesItems(t *testing.T) {
	var got *http.Request
	srv := serveFile(t, "testdata/receipt.json", func(r *http.Request) {
		_ = r.ParseForm()
		got = r
	})
	defer srv.Close()

	res, err := New(srv.URL, "секрет", time.Second).Enrich(context.Background(), fiscalReceipt())
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Items) != 3 {
		t.Fatalf("позиций %d, ожидалось 3", len(res.Items))
	}
	if res.Merchant != "Пятёрочка" {
		t.Errorf("магазин %q", res.Merchant)
	}
	if res.Items[0].Name != "МОЛОКО ПРОСТОКВ.3,2% 930МЛ" {
		t.Errorf("название %q", res.Items[0].Name)
	}
	if want := domain.MustParseMoney("89.00"); res.Items[0].Sum != want {
		t.Errorf("сумма %s, ожидалась %s", res.Items[0].Sum, want)
	}
	if res.Items[2].Count != 0.831 {
		t.Errorf("вес %v, ожидалось 0.831", res.Items[2].Count)
	}

	// Реквизиты чека должны уходить в запрос: по ним чек и ищется.
	for field, want := range map[string]string{
		"fn": "9960440300123456", "fd": "12345", "fp": "1234567890",
		"token": "секрет", "s": "154.32", "n": "1",
	} {
		if v := got.Form.Get(field); v != want {
			t.Errorf("поле %s = %q, ожидалось %q", field, v, want)
		}
	}
}

// Ручные траты фискальных реквизитов не имеют, искать по ним нечего.
func TestEnrichSkipsNonFiscal(t *testing.T) {
	if _, err := New("http://unused", "t", time.Second).Enrich(context.Background(), domain.Receipt{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("ожидалась ErrNotFound, получено %v", err)
	}
}

func TestEnrichHandlesServiceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "t", time.Second).Enrich(context.Background(), fiscalReceipt()); err == nil {
		t.Error("ошибка сервиса должна возвращаться")
	}
}

// Чек может ещё не дойти от кассы до базы — это не поломка.
func TestEnrichHandlesNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":null}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "t", time.Second).Enrich(context.Background(), fiscalReceipt()); !errors.Is(err, ErrNotFound) {
		t.Errorf("ожидалась ErrNotFound, получено %v", err)
	}
}

// Не все кассы заполняют сумму позиции.
func TestResponseComputesMissingSum(t *testing.T) {
	var r response
	r.Code = codeOK
	r.Data.JSON.Items = append(r.Data.JSON.Items, struct {
		Name     string  `json:"name"`
		Price    int64   `json:"price"`
		Quantity float64 `json:"quantity"`
		Sum      int64   `json:"sum"`
	}{Name: "СЫР", Price: 20000, Quantity: 2, Sum: 0})

	res, err := r.toResult()
	if err != nil {
		t.Fatal(err)
	}
	if want := domain.MustParseMoney("400.00"); res.Items[0].Sum != want {
		t.Errorf("сумма %s, ожидалась %s", res.Items[0].Sum, want)
	}
}
