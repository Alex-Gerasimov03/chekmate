package postgres

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Alex-Gerasimov03/chekmate/internal/storage"
)

// Тесты идут против настоящего PostgreSQL: заглушка не проверит ни частичный
// уникальный индекс, ни ON CONFLICT, ради которых схема и написана.
var (
	pool *pgxpool.Pool
	dsn  string
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		return
	}
	os.Exit(runSuite(m))
}

// runSuite вынесен из TestMain, чтобы os.Exit не отменял отложенную
// остановку контейнера: иначе postgres оставался бы висеть после ошибки.
func runSuite(m *testing.M) int {
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("chekmate"),
		tcpostgres.WithUsername("chekmate"),
		tcpostgres.WithPassword("chekmate"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "не удалось поднять postgres: %v\n", err)
		return 1
	}
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	if dsn, err = ctr.ConnectionString(ctx, "sslmode=disable"); err != nil {
		fmt.Fprintf(os.Stderr, "строка подключения: %v\n", err)
		return 1
	}
	if err := storage.Migrate(ctx, dsn); err != nil {
		fmt.Fprintf(os.Stderr, "миграции: %v\n", err)
		return 1
	}
	if pool, err = NewPool(ctx, dsn); err != nil {
		fmt.Fprintf(os.Stderr, "пул: %v\n", err)
		return 1
	}
	defer pool.Close()

	return m.Run()
}

// newBudget заводит отдельный бюджет на каждый тест, чтобы они не мешали друг другу.
func newBudget(t *testing.T) int64 {
	t.Helper()
	id, err := NewBudgets(pool).GetOrCreate(context.Background(), time.Now().UnixNano(), "Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// Обратная миграция должна работать: без неё откат выкатки невозможен.
func TestMigrationsAreReversible(t *testing.T) {
	if err := storage.Rollback(context.Background(), dsn); err != nil {
		t.Fatalf("откат миграций: %v", err)
	}
	if err := storage.Migrate(context.Background(), dsn); err != nil {
		t.Fatalf("повторное применение: %v", err)
	}
}
