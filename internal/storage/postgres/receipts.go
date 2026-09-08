package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

const (
	SourceQR     = "qr"
	SourceManual = "manual"
)

type Receipts struct{ pool *pgxpool.Pool }

func NewReceipts(pool *pgxpool.Pool) *Receipts { return &Receipts{pool: pool} }

// Save записывает чек вместе с позициями.
func (r *Receipts) Save(ctx context.Context, budgetID, userID int64, source string, rc domain.Receipt) (id int64, created bool, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("начало транзакции: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var fn, fd, fp *string
	if rc.IsFiscal() {
		fn, fd, fp = &rc.FN, &rc.FD, &rc.FP
	}

	const insert = `
		INSERT INTO receipts (budget_id, user_id, fn, fd, fp, total, bought_at, merchant, source, raw_qr)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (fn, fd, fp) WHERE fn IS NOT NULL DO NOTHING
		RETURNING id`

	err = tx.QueryRow(ctx, insert,
		budgetID, userID, fn, fd, fp,
		int64(rc.Total), rc.At, rc.Merchant, source, rc.Raw,
	).Scan(&id)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		const existing = `SELECT id FROM receipts WHERE fn = $1 AND fd = $2 AND fp = $3`
		if err := tx.QueryRow(ctx, existing, fn, fd, fp).Scan(&id); err != nil {
			return 0, false, fmt.Errorf("поиск ранее добавленного чека: %w", err)
		}
		return id, false, tx.Commit(ctx)
	case err != nil:
		return 0, false, fmt.Errorf("сохранение чека: %w", err)
	}

	if err := insertItems(ctx, tx, id, rc.Items); err != nil {
		return 0, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, false, fmt.Errorf("фиксация транзакции: %w", err)
	}
	return id, true, nil
}

func insertItems(ctx context.Context, tx pgx.Tx, receiptID int64, items []domain.Item) error {
	if len(items) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(items))
	for _, it := range items {
		var productID *int64
		if it.ProductID != 0 {
			id := it.ProductID
			productID = &id
		}
		rows = append(rows, []any{
			receiptID, it.RawName, productID, it.Qty.Milli(), string(it.Qty.Unit()),
			int64(it.Sum), int64(it.UnitPrice),
		})
	}

	_, err := tx.CopyFrom(ctx,
		pgx.Identifier{"receipt_items"},
		[]string{"receipt_id", "raw_name", "product_id", "qty_milli", "unit", "sum", "unit_price"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("сохранение позиций: %w", err)
	}
	return nil
}

// CountByBudget используется отчётами и тестами.
func (r *Receipts) CountByBudget(ctx context.Context, budgetID int64) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM receipts WHERE budget_id = $1`, budgetID).Scan(&n)
	return n, err
}

// AssignProduct привязывает позицию к товару после ответа пользователя.
func (r *Receipts) AssignProduct(ctx context.Context, receiptID int64, rawName string, productID int64) error {
	const q = `
		UPDATE receipt_items SET product_id = $3
		WHERE receipt_id = $1 AND raw_name = $2 AND product_id IS NULL`

	if _, err := r.pool.Exec(ctx, q, receiptID, rawName, productID); err != nil {
		return fmt.Errorf("привязка позиции к товару: %w", err)
	}
	return nil
}
