// Package storage хранит схему базы и её применение.
package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate применяет схему. Миграции лежат внутри бинарника, поэтому деплой
// не требует ничего, кроме самого файла.
func Migrate(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("подключение для миграций: %w", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("применение миграций: %w", err)
	}
	return nil
}

// Rollback откатывает схему целиком. Нужен для проверки того, что обратные
// миграции написаны и работают: без них откат выкатки невозможен.
func Rollback(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("подключение для отката: %w", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.DownToContext(ctx, db, "migrations", 0); err != nil {
		return fmt.Errorf("откат миграций: %w", err)
	}
	return nil
}
