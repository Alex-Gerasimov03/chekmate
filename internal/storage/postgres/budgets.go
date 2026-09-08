package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Budgets struct{ pool *pgxpool.Pool }

func NewBudgets(pool *pgxpool.Pool) *Budgets { return &Budgets{pool: pool} }

// GetOrCreate возвращает бюджет чата, заводя его при первом обращении.
func (b *Budgets) GetOrCreate(ctx context.Context, chatID int64, tz string) (int64, error) {
	const q = `
		INSERT INTO budgets (chat_id, timezone) VALUES ($1, $2)
		ON CONFLICT (chat_id) DO UPDATE SET chat_id = EXCLUDED.chat_id
		RETURNING id`

	var id int64
	if err := b.pool.QueryRow(ctx, q, chatID, tz).Scan(&id); err != nil {
		return 0, fmt.Errorf("бюджет чата %d: %w", chatID, err)
	}
	return id, nil
}
