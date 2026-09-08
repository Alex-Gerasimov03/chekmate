package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alex-Gerasimov03/chekmate/internal/catalog"
	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

type Products struct{ pool *pgxpool.Pool }

func NewProducts(pool *pgxpool.Pool) *Products { return &Products{pool: pool} }

func (p *Products) AliasProduct(ctx context.Context, raw string) (int64, bool, error) {
	var id int64
	err := p.pool.QueryRow(ctx,
		`SELECT product_id FROM product_aliases WHERE raw_name = $1`, raw).Scan(&id)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return 0, false, nil
	case err != nil:
		return 0, false, fmt.Errorf("чтение алиаса: %w", err)
	}
	return id, true, nil
}

// FindSimilar ищет товары с похожим названием той же единицы измерения.
// Отбор по сходству делает Postgres: триграммный индекс для этого и заведён.
func (p *Products) FindSimilar(ctx context.Context, canonical string, unit domain.Unit, fat int) ([]catalog.Candidate, error) {
	const q = `
		SELECT id, title, similarity(canonical_name, $1) AS s
		FROM products
		WHERE canonical_name % $1
		  AND unit = $2
		  AND ($3 = 0 OR fat = 0 OR fat = $3)
		ORDER BY s DESC
		LIMIT 5`

	rows, err := p.pool.Query(ctx, q, canonical, string(unit), fat)
	if err != nil {
		return nil, fmt.Errorf("поиск похожих: %w", err)
	}
	defer rows.Close()

	var out []catalog.Candidate
	for rows.Next() {
		var c catalog.Candidate
		var sim float32
		if err := rows.Scan(&c.ProductID, &c.Title, &sim); err != nil {
			return nil, err
		}
		c.Similarity = float64(sim)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (p *Products) CreateProduct(ctx context.Context, np catalog.NewProduct) (int64, error) {
	const q = `
		INSERT INTO products (canonical_name, title, fat, unit, pack_milli, dict_version)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (canonical_name, fat, unit, pack_milli)
		DO UPDATE SET title = products.title
		RETURNING id`

	var id int64
	err := p.pool.QueryRow(ctx, q,
		np.Canonical, np.Title, np.Fat, string(np.Unit), np.PackMilli, np.Version).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("создание товара: %w", err)
	}
	return id, nil
}

// SaveAlias не понижает подтверждённость: ответ пользователя весомее догадки.
func (p *Products) SaveAlias(ctx context.Context, raw string, productID int64, confirmed bool) error {
	const q = `
		INSERT INTO product_aliases (raw_name, product_id, confirmed)
		VALUES ($1, $2, $3)
		ON CONFLICT (raw_name) DO UPDATE
		SET product_id = EXCLUDED.product_id,
		    confirmed  = product_aliases.confirmed OR EXCLUDED.confirmed`

	if _, err := p.pool.Exec(ctx, q, raw, productID, confirmed); err != nil {
		return fmt.Errorf("сохранение алиаса: %w", err)
	}
	return nil
}

// SetCategory привязывает товар к категории по словарю.
func (p *Products) SetCategory(ctx context.Context, productID, categoryID int64) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE products SET category_id = $2, category_source = 'auto' WHERE id = $1`,
		productID, categoryID)
	return err
}

// SetCategoryByUser закрепляет выбор человека: словарь его больше не тронет.
func (p *Products) SetCategoryByUser(ctx context.Context, productID, categoryID int64) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE products SET category_id = $2, category_source = 'user' WHERE id = $1`,
		productID, categoryID)
	return err
}

// ListByBudget отдаёт товары, которые бюджет реально покупал. Нужен для
// пересчёта категорий после добавления правила.
func (p *Products) ListByBudget(ctx context.Context, budgetID int64) ([]catalog.ProductRef, error) {
	const q = `
		SELECT DISTINCT pr.id, pr.canonical_name
		FROM products pr
		JOIN receipt_items i ON i.product_id = pr.id
		JOIN receipts r ON r.id = i.receipt_id
		WHERE r.budget_id = $1`

	rows, err := p.pool.Query(ctx, q, budgetID)
	if err != nil {
		return nil, fmt.Errorf("товары бюджета: %w", err)
	}
	defer rows.Close()

	var out []catalog.ProductRef
	for rows.Next() {
		var ref catalog.ProductRef
		if err := rows.Scan(&ref.ID, &ref.Canonical); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// FallbackCategory — куда попадают товары, которые словарь не опознал.
// Без него трата есть, а в отчёте её нет: разбивка строится по категориям.
const FallbackCategory = "other"

// Recategorize пересматривает категории, проставленные словарём.
func (p *Products) Recategorize(ctx context.Context, cats *catalog.Categorizer) (changed, unknown int, err error) {
	var fallbackID int64
	if err := p.pool.QueryRow(ctx,
		`SELECT id FROM categories WHERE code = $1`, FallbackCategory).Scan(&fallbackID); err != nil {
		return 0, 0, fmt.Errorf("категория %q: %w", FallbackCategory, err)
	}

	const q = `
		SELECT id, canonical_name, category_id
		FROM products
		WHERE category_source = 'auto' OR category_id IS NULL`

	rows, err := p.pool.Query(ctx, q)
	if err != nil {
		return 0, 0, fmt.Errorf("товары без категории: %w", err)
	}

	type item struct {
		ref     catalog.ProductRef
		current *int64
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ref.ID, &it.ref.Canonical, &it.current); err != nil {
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
		categoryID, ok := cats.Categorize(it.ref.Canonical)
		if !ok {
			categoryID = fallbackID
			unknown++
		}
		if it.current != nil && *it.current == categoryID {
			continue
		}
		if err := p.SetCategory(ctx, it.ref.ID, categoryID); err != nil {
			return changed, unknown, err
		}
		changed++
	}
	return changed, unknown, nil
}

// FallbackCategoryID отдаёт категорию для товаров, которые словарь не опознал.
func (p *Products) FallbackCategoryID(ctx context.Context) (int64, error) {
	var id int64
	err := p.pool.QueryRow(ctx,
		`SELECT id FROM categories WHERE code = $1`, FallbackCategory).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("категория %q: %w", FallbackCategory, err)
	}
	return id, nil
}
