package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

// Client получает состав чека по фискальным реквизитам.
type Client struct {
	endpoint string
	token    string
	http     *http.Client
}

func New(endpoint, token string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Client{
		endpoint: endpoint,
		token:    token,
		http:     &http.Client{Timeout: timeout},
	}
}

// Enrich запрашивает состав чека. Реквизиты берутся из QR-кода: по ним чек
// и находится в базе фискальных данных.
func (c *Client) Enrich(ctx context.Context, r domain.Receipt) (Result, error) {
	if !r.IsFiscal() {
		return Result{}, ErrNotFound
	}

	form := url.Values{
		"token": {c.token},
		"fn":    {r.FN},
		"fd":    {r.FD},
		"fp":    {r.FP},
		"t":     {r.At.Format("20060102T150405")},
		"s":     {strconv.FormatFloat(absRubles(r.Total), 'f', 2, 64)},
		"n":     {strconv.Itoa(int(r.Operation))},
		"qr":    {"0"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Result{}, fmt.Errorf("запрос состава чека: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("запрос состава чека: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("состав чека: сервис ответил %s", resp.Status)
	}

	var payload response
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Result{}, fmt.Errorf("разбор ответа сервиса: %w", err)
	}
	return payload.toResult()
}

func absRubles(m domain.Money) float64 {
	if m < 0 {
		m = -m
	}
	return m.Rubles()
}
