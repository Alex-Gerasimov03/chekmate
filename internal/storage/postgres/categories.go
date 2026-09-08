package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alex-Gerasimov03/chekmate/internal/analytics"
)

type Categories struct{ pool *pgxpool.Pool }

func NewCategories(pool *pgxpool.Pool) *Categories { return &Categories{pool: pool} }

var ErrNoCategory = errors.New("категория не найдена")

func (c *Categories) ByCode(ctx context.Context, code string) (int64, error) {
	var id int64
	err := c.pool.QueryRow(ctx, `SELECT id FROM categories WHERE code = $1`, code).Scan(&id)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoCategory
	}
	if err != nil {
		return 0, fmt.Errorf("поиск категории: %w", err)
	}
	return id, nil
}

func (c *Categories) All(ctx context.Context) ([]analytics.Category, error) {
	rows, err := c.pool.Query(ctx, `SELECT id, code, title, is_food FROM categories ORDER BY is_food DESC, title`)
	if err != nil {
		return nil, fmt.Errorf("справочник категорий: %w", err)
	}
	defer rows.Close()

	var out []analytics.Category
	for rows.Next() {
		var cat analytics.Category
		if err := rows.Scan(&cat.ID, &cat.Code, &cat.Title, &cat.IsFood); err != nil {
			return nil, err
		}
		out = append(out, cat)
	}
	return out, rows.Err()
}
