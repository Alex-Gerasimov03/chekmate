package httpsrv

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func handler(t *testing.T, checks ...func(context.Context) error) http.Handler {
	t.Helper()

	named := map[string]func(context.Context) error{}
	for i, c := range checks {
		named[fmt.Sprintf("проверка%d", i)] = c
	}
	return New(":0", named, slog.Default()).srv.Handler
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("код %d, ожидался 200", rec.Code)
	}
}

// Живой процесс без базы обслуживать запросы не может, и балансировщик
// должен об этом знать.
func TestReadyzReportsBrokenDatabase(t *testing.T) {
	rec := httptest.NewRecorder()
	broken := func(context.Context) error { return errors.New("нет соединения") }
	handler(t, broken).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("код %d, ожидался 503", rec.Code)
	}
}

func TestReadyzOK(t *testing.T) {
	rec := httptest.NewRecorder()
	ok := func(context.Context) error { return nil }
	handler(t, ok).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("код %d, ожидался 200", rec.Code)
	}
}

func TestMetrics(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидался 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if len(body) == 0 {
		t.Error("метрики пусты")
	}
}

// Готовность падает, если отказала хотя бы одна зависимость: при живой базе
// и мёртвом Telegram бот сообщений всё равно не получает.
func TestReadyzFailsOnAnyCheck(t *testing.T) {
	ok := func(context.Context) error { return nil }
	broken := func(context.Context) error { return errors.New("нет ответа от Telegram") }

	rec := httptest.NewRecorder()
	handler(t, ok, broken).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("код %d, ожидался 503", rec.Code)
	}
}
