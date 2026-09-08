package enrich

import (
	"fmt"
	"strings"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

// Формат задаёт сторонний сервис: при его изменении правится только этот
// файл. Суммы приходят в копейках — так же, как хранятся у нас.
type response struct {
	Code int `json:"code"`
	Data struct {
		JSON struct {
			Items []struct {
				Name     string  `json:"name"`
				Price    int64   `json:"price"`
				Quantity float64 `json:"quantity"`
				Sum      int64   `json:"sum"`
			} `json:"items"`
			RetailPlace     string `json:"retailPlace"`
			RetailPlaceAddr string `json:"retailPlaceAddress"`
			User            string `json:"user"`
		} `json:"json"`
	} `json:"data"`
}

// codeOK — признак успешного ответа сервиса.
const codeOK = 1

func (r response) toResult() (Result, error) {
	if r.Code != codeOK {
		return Result{}, fmt.Errorf("%w: сервис вернул код %d", ErrNotFound, r.Code)
	}

	items := make([]RawItem, 0, len(r.Data.JSON.Items))
	for _, it := range r.Data.JSON.Items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			continue
		}

		count := it.Quantity
		if count <= 0 {
			count = 1
		}

		sum := domain.Money(it.Sum)
		if sum == 0 {
			// Не все кассы заполняют сумму позиции: считаем из цены и количества.
			sum = domain.Money(float64(it.Price)*count + 0.5)
		}

		items = append(items, RawItem{Name: name, Count: count, Sum: sum})
	}

	if len(items) == 0 {
		return Result{}, ErrNotFound
	}
	return Result{Items: items, Merchant: merchantOf(r)}, nil
}

// merchantOf выбирает наиболее человеческое из названий продавца: в чеках
// заполнено то одно поле, то другое.
func merchantOf(r response) string {
	for _, name := range []string{r.Data.JSON.RetailPlace, r.Data.JSON.User, r.Data.JSON.RetailPlaceAddr} {
		if name = strings.TrimSpace(name); name != "" {
			return name
		}
	}
	return ""
}
