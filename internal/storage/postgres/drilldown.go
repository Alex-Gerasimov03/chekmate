package postgres

import (
	"context"
	"fmt"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

// ProductsInCategory — на что именно ушли деньги внутри категории.
// Это следующий уровень детализации после отчёта по категориям.
func (a *Analytics) ProductsInCategory(ctx context.Context, budgetID, categoryID int64, p analytics.Period, limit int) ([]analytics.ProductSpend, error) {
	const q = `
		SELECT pr.id, pr.title, sum(i.sum), count(*),
		       (array_agg(i.unit_price ORDER BY r.bought_at DESC))[1]
		FROM receipt_items i
		JOIN receipts r ON r.id = i.receipt_id
		JOIN products pr ON pr.id = i.product_id
		WHERE r.budget_id = $1 AND pr.category_id = $2
		  AND r.bought_at >= $3 AND r.bought_at < $4
		GROUP BY pr.id, pr.title
		ORDER BY sum(i.sum) DESC
		LIMIT $5`

	rows, err := a.pool.Query(ctx, q, budgetID, categoryID, p.From, p.To, limit)
	if err != nil {
		return nil, fmt.Errorf("товары категории: %w", err)
	}
	defer rows.Close()

	var out []analytics.ProductSpend
	for rows.Next() {
		var ps analytics.ProductSpend
		var spend, last int64
		if err := rows.Scan(&ps.ProductID, &ps.Title, &spend, &ps.Purchases, &last); err != nil {
			return nil, err
		}
		ps.Spend, ps.LastPrice = domain.Money(spend), domain.Money(last)
		out = append(out, ps)
	}
	return out, rows.Err()
}

// ReceiptsInPeriod — история покупок: список чеков от свежих к старым.
func (a *Analytics) ReceiptsInPeriod(ctx context.Context, budgetID int64, p analytics.Period, limit, offset int) ([]analytics.ReceiptSummary, error) {
	const q = `
		SELECT r.id, r.bought_at, r.total, r.merchant,
		       (SELECT count(*) FROM receipt_items i WHERE i.receipt_id = r.id)
		FROM receipts r
		WHERE r.budget_id = $1 AND r.bought_at >= $2 AND r.bought_at < $3
		ORDER BY r.bought_at DESC
		LIMIT $4 OFFSET $5`

	rows, err := a.pool.Query(ctx, q, budgetID, p.From, p.To, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("история покупок: %w", err)
	}
	defer rows.Close()

	var out []analytics.ReceiptSummary
	for rows.Next() {
		var rs analytics.ReceiptSummary
		var total int64
		if err := rows.Scan(&rs.ID, &rs.At, &total, &rs.Merchant, &rs.Items); err != nil {
			return nil, err
		}
		rs.Total = domain.Money(total)
		out = append(out, rs)
	}
	return out, rows.Err()
}

// CountReceipts нужен, чтобы показать, есть ли ещё страницы истории.
func (a *Analytics) CountReceipts(ctx context.Context, budgetID int64, p analytics.Period) (int, error) {
	var n int
	err := a.pool.QueryRow(ctx,
		`SELECT count(*) FROM receipts WHERE budget_id = $1 AND bought_at >= $2 AND bought_at < $3`,
		budgetID, p.From, p.To).Scan(&n)
	return n, err
}

// ReceiptLines — позиции одного чека, самый нижний уровень детализации.
func (a *Analytics) ReceiptLines(ctx context.Context, budgetID, receiptID int64) (analytics.ReceiptSummary, []analytics.ReceiptLine, error) {
	var head analytics.ReceiptSummary
	var total int64

	err := a.pool.QueryRow(ctx, `
		SELECT id, bought_at, total, merchant
		FROM receipts WHERE id = $1 AND budget_id = $2`,
		receiptID, budgetID).Scan(&head.ID, &head.At, &total, &head.Merchant)
	if err != nil {
		return head, nil, fmt.Errorf("чек %d: %w", receiptID, err)
	}
	head.Total = domain.Money(total)

	const q = `
		SELECT i.raw_name, coalesce(pr.title, ''), coalesce(c.title, ''),
		       i.qty_milli, i.unit, i.sum, i.unit_price
		FROM receipt_items i
		LEFT JOIN products pr ON pr.id = i.product_id
		LEFT JOIN categories c ON c.id = pr.category_id
		WHERE i.receipt_id = $1
		ORDER BY i.sum DESC`

	rows, err := a.pool.Query(ctx, q, receiptID)
	if err != nil {
		return head, nil, fmt.Errorf("позиции чека: %w", err)
	}
	defer rows.Close()

	var out []analytics.ReceiptLine
	for rows.Next() {
		var l analytics.ReceiptLine
		var sum, price int64
		if err := rows.Scan(&l.RawName, &l.Title, &l.Category, &l.Qty.Milli, &l.Qty.Unit, &sum, &price); err != nil {
			return head, nil, err
		}
		l.Sum, l.UnitPrice = domain.Money(sum), domain.Money(price)
		out = append(out, l)
	}
	head.Items = len(out)
	return head, out, rows.Err()
}

// ProductTitle нужен, когда в товар провалились из списка и название
// показывать неоткуда.
func (a *Analytics) ProductTitle(ctx context.Context, productID int64) (string, error) {
	var title string
	err := a.pool.QueryRow(ctx, `SELECT title FROM products WHERE id = $1`, productID).Scan(&title)
	if err != nil {
		return "", fmt.Errorf("название товара: %w", err)
	}
	return title, nil
}
