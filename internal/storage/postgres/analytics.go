package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

type Analytics struct{ pool *pgxpool.Pool }

func NewAnalytics(pool *pgxpool.Pool) *Analytics { return &Analytics{pool: pool} }

// CategorySpends считает траты по категориям сразу за оба периода: одним
// проходом вместо двух запросов и склейки в приложении.
func (a *Analytics) CategorySpends(ctx context.Context, budgetID int64, cur, prev analytics.Period) ([]analytics.CategorySpend, error) {
	const q = `
		WITH spent AS (
		    SELECT p.category_id,
		           sum(i.sum) FILTER (WHERE r.bought_at >= $2 AND r.bought_at < $3) AS current,
		           sum(i.sum) FILTER (WHERE r.bought_at >= $4 AND r.bought_at < $5) AS previous
		    FROM receipts r
		    JOIN receipt_items i ON i.receipt_id = r.id
		    JOIN products p ON p.id = i.product_id
		    WHERE r.budget_id = $1 AND r.bought_at >= $4 AND r.bought_at < $3
		    GROUP BY p.category_id
		)
		SELECT c.id, c.code, c.title, c.is_food,
		       coalesce(s.current, 0), coalesce(s.previous, 0)
		FROM categories c
		LEFT JOIN spent s ON s.category_id = c.id`

	rows, err := a.pool.Query(ctx, q, budgetID, cur.From, cur.To, prev.From, prev.To)
	if err != nil {
		return nil, fmt.Errorf("траты по категориям: %w", err)
	}
	defer rows.Close()

	var out []analytics.CategorySpend
	for rows.Next() {
		var s analytics.CategorySpend
		var current, previous int64
		if err := rows.Scan(&s.CategoryID, &s.Code, &s.Title, &s.IsFood, &current, &previous); err != nil {
			return nil, err
		}
		s.Current, s.Previous = domain.Money(current), domain.Money(previous)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return a.addUnaccounted(ctx, budgetID, cur, prev, out)
}

// addUnaccounted досыпает в "Прочее" траты, не разложенные по позициям:
func (a *Analytics) addUnaccounted(ctx context.Context, budgetID int64, cur, prev analytics.Period, rows []analytics.CategorySpend) ([]analytics.CategorySpend, error) {
	const q = `
		SELECT coalesce(sum(r.total) FILTER (WHERE r.bought_at >= $2 AND r.bought_at < $3), 0)
		     - coalesce(sum(i.covered) FILTER (WHERE r.bought_at >= $2 AND r.bought_at < $3), 0),
		       coalesce(sum(r.total) FILTER (WHERE r.bought_at >= $4 AND r.bought_at < $5), 0)
		     - coalesce(sum(i.covered) FILTER (WHERE r.bought_at >= $4 AND r.bought_at < $5), 0)
		FROM receipts r
		LEFT JOIN LATERAL (
		    SELECT coalesce(sum(it.sum), 0) AS covered
		    FROM receipt_items it
		    WHERE it.receipt_id = r.id AND it.product_id IS NOT NULL
		) i ON true
		WHERE r.budget_id = $1 AND r.bought_at >= $4 AND r.bought_at < $3`

	var current, previous int64
	if err := a.pool.QueryRow(ctx, q, budgetID, cur.From, cur.To, prev.From, prev.To).
		Scan(&current, &previous); err != nil {
		return nil, fmt.Errorf("неразложенные траты: %w", err)
	}
	if current == 0 && previous == 0 {
		return rows, nil
	}

	for i := range rows {
		if rows[i].Code == analytics.FallbackCategory {
			rows[i].Current += domain.Money(current)
			rows[i].Previous += domain.Money(previous)
			return rows, nil
		}
	}
	return rows, nil
}

func (a *Analytics) ReceiptCount(ctx context.Context, budgetID int64, p analytics.Period) (int, error) {
	const q = `
		SELECT count(*) FROM receipts
		WHERE budget_id = $1 AND bought_at >= $2 AND bought_at < $3`

	var n int
	err := a.pool.QueryRow(ctx, q, budgetID, p.From, p.To).Scan(&n)
	return n, err
}

// PriceObservations даёт медианную цену за базовую единицу по каждому товару.
// Медиана, а не среднее: покупка по акции не должна утаскивать индекс вниз.
func (a *Analytics) PriceObservations(ctx context.Context, budgetID int64, p analytics.Period) ([]analytics.PriceObservation, error) {
	const q = `
		SELECT i.product_id,
		       coalesce(pr.category_id, 0),
		       pr.title,
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY i.unit_price)::bigint,
		       sum(i.sum),
		       count(*)
		FROM receipt_items i
		JOIN receipts r ON r.id = i.receipt_id
		JOIN products pr ON pr.id = i.product_id
		WHERE r.budget_id = $1 AND r.bought_at >= $2 AND r.bought_at < $3
		GROUP BY i.product_id, pr.category_id, pr.title`

	rows, err := a.pool.Query(ctx, q, budgetID, p.From, p.To)
	if err != nil {
		return nil, fmt.Errorf("наблюдения цен: %w", err)
	}
	defer rows.Close()

	var out []analytics.PriceObservation
	for rows.Next() {
		var o analytics.PriceObservation
		var price, spend int64
		if err := rows.Scan(&o.ProductID, &o.CategoryID, &o.Title, &price, &spend, &o.Purchases); err != nil {
			return nil, err
		}
		o.UnitPrice, o.Spend = domain.Money(price), domain.Money(spend)
		out = append(out, o)
	}
	return out, rows.Err()
}

// PriceHistory возвращает историю цены товара, самые свежие покупки первыми.
func (a *Analytics) PriceHistory(ctx context.Context, budgetID, productID int64, limit int) ([]analytics.PricePoint, error) {
	const q = `
		SELECT to_char(r.bought_at, 'DD.MM.YYYY'), r.merchant, i.unit_price
		FROM receipt_items i
		JOIN receipts r ON r.id = i.receipt_id
		WHERE r.budget_id = $1 AND i.product_id = $2
		ORDER BY r.bought_at DESC
		LIMIT $3`

	rows, err := a.pool.Query(ctx, q, budgetID, productID, limit)
	if err != nil {
		return nil, fmt.Errorf("история цены: %w", err)
	}
	defer rows.Close()

	var out []analytics.PricePoint
	for rows.Next() {
		var p analytics.PricePoint
		var price int64
		if err := rows.Scan(&p.At, &p.Merchant, &price); err != nil {
			return nil, err
		}
		p.UnitPrice = domain.Money(price)
		out = append(out, p)
	}
	return out, rows.Err()
}

// SearchProducts ищет товары бюджета по части названия — для команды /price.
func (a *Analytics) SearchProducts(ctx context.Context, budgetID int64, query string, limit int) ([]analytics.PriceObservation, error) {
	const q = `
		SELECT DISTINCT p.id, coalesce(p.category_id, 0), p.title
		FROM products p
		JOIN receipt_items i ON i.product_id = p.id
		JOIN receipts r ON r.id = i.receipt_id
		WHERE r.budget_id = $1 AND p.title ILIKE '%' || $2 || '%'
		ORDER BY p.title
		LIMIT $3`

	rows, err := a.pool.Query(ctx, q, budgetID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("поиск товаров: %w", err)
	}
	defer rows.Close()

	var out []analytics.PriceObservation
	for rows.Next() {
		var o analytics.PriceObservation
		if err := rows.Scan(&o.ProductID, &o.CategoryID, &o.Title); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// CategoryTitles — справочник для подписей в отчётах.
func (a *Analytics) CategoryTitles(ctx context.Context) (map[int64]string, error) {
	rows, err := a.pool.Query(ctx, `SELECT id, title FROM categories`)
	if err != nil {
		return nil, fmt.Errorf("справочник категорий: %w", err)
	}
	defer rows.Close()

	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, err
		}
		out[id] = title
	}
	return out, rows.Err()
}
