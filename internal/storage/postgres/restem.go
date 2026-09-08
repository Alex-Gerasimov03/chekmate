package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
	"github.com/Alex-Gerasimov03/chekmate/internal/normalize"
)

// unitOf повторяет подстановку из каталога: товар без веса и объёма
// в названии считается штучным.
func unitOf(res normalize.Result) string {
	if u := res.Qty.Unit(); u != "" {
		return string(u)
	}
	return string(domain.UnitPcs)
}

// Restem пересчитывает канонические ключи товаров после изменения словаря.
func (p *Products) Restem(ctx context.Context, norm *normalize.Normalizer) (updated, merged int, err error) {
	const outdated = `
		SELECT pr.id, coalesce(
		    (SELECT a.raw_name FROM product_aliases a
		      WHERE a.product_id = pr.id ORDER BY a.confirmed DESC LIMIT 1),
		    pr.title)
		FROM products pr
		WHERE pr.dict_version <> $1`

	rows, err := p.pool.Query(ctx, outdated, norm.Version())
	if err != nil {
		return 0, 0, fmt.Errorf("поиск устаревших товаров: %w", err)
	}

	type item struct {
		id  int64
		raw string
	}
	var items []item

	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.raw); err != nil {
			rows.Close()
			return 0, 0, err
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	for _, it := range items {
		res := norm.Name(it.raw)
		if res.Canonical == "" {
			continue
		}

		wasMerged, err := p.restemOne(ctx, it.id, res, norm.Version())
		if err != nil {
			return updated, merged, err
		}
		if wasMerged {
			merged++
		} else {
			updated++
		}
	}
	return updated, merged, nil
}

// restemOne приводит один товар к новому ключу, сливая его с уже
// существующим двойником, если тот появился.
func (p *Products) restemOne(ctx context.Context, id int64, res normalize.Result, version int) (bool, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const findTwin = `
		SELECT id FROM products
		WHERE canonical_name = $1 AND fat = $2 AND unit = $3 AND pack_milli = $4 AND id <> $5`

	var twin int64
	err = tx.QueryRow(ctx, findTwin,
		res.Canonical, res.Fat, unitOf(res), res.Qty.Milli(), id).Scan(&twin)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		const update = `
			UPDATE products
			SET canonical_name = $2, title = $3, fat = $4, unit = $5,
			    pack_milli = $6, dict_version = $7
			WHERE id = $1`

		if _, err := tx.Exec(ctx, update, id, res.Canonical, res.Title,
			res.Fat, unitOf(res), res.Qty.Milli(), version); err != nil {
			return false, fmt.Errorf("обновление товара %d: %w", id, err)
		}
		return false, tx.Commit(ctx)

	case err != nil:
		return false, fmt.Errorf("поиск двойника товара %d: %w", id, err)
	}

	// Двойник существует — переносим на него всё и убираем дубликат.
	for _, q := range []string{
		`UPDATE receipt_items SET product_id = $2 WHERE product_id = $1`,
		`UPDATE product_aliases SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, id, twin); err != nil {
			return false, fmt.Errorf("перенос на товар %d: %w", twin, err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE products SET dict_version = $2 WHERE id = $1`, twin, version); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM products WHERE id = $1`, id); err != nil {
		return false, fmt.Errorf("удаление дубликата %d: %w", id, err)
	}
	return true, tx.Commit(ctx)
}
