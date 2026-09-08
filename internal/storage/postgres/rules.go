package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alex-Gerasimov03/chekmate/internal/catalog"
)

type Rules struct{ pool *pgxpool.Pool }

func NewRules(pool *pgxpool.Pool) *Rules { return &Rules{pool: pool} }

// Load читает общие правила и личные правила бюджета.
func (r *Rules) Load(ctx context.Context, budgetID int64) ([]catalog.Rule, error) {
	const q = `
		SELECT word, category_id, budget_id IS NOT NULL, priority
		FROM category_rules
		WHERE budget_id IS NULL OR budget_id = $1`

	rows, err := r.pool.Query(ctx, q, budgetID)
	if err != nil {
		return nil, fmt.Errorf("чтение правил категоризации: %w", err)
	}
	defer rows.Close()

	var out []catalog.Rule
	for rows.Next() {
		var rule catalog.Rule
		if err := rows.Scan(&rule.Word, &rule.CategoryID, &rule.Personal, &rule.Priority); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// Add сохраняет правило, заданное пользователем из бота.
func (r *Rules) Add(ctx context.Context, budgetID, categoryID int64, word string) error {
	const q = `
		INSERT INTO category_rules (word, category_id, budget_id, priority)
		VALUES ($1, $2, $3, 100)
		ON CONFLICT (word, budget_id) DO UPDATE SET category_id = EXCLUDED.category_id`

	if _, err := r.pool.Exec(ctx, q, word, categoryID, budgetID); err != nil {
		return fmt.Errorf("сохранение правила: %w", err)
	}
	return nil
}
