// Команда меряет точность категоризации на размеченном наборе строк чеков.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alex-Gerasimov03/chekmate/internal/catalog"
	"github.com/Alex-Gerasimov03/chekmate/internal/config"
	"github.com/Alex-Gerasimov03/chekmate/internal/normalize"
	"github.com/Alex-Gerasimov03/chekmate/internal/storage/postgres"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	path := "testdata/receipt_lines.csv"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	dict, err := postgres.NewDictionaries(pool).Load(ctx)
	if err != nil {
		return err
	}
	rules, err := postgres.NewRules(pool).Load(ctx, 0)
	if err != nil {
		return err
	}
	codes, err := categoryCodes(ctx, pool)
	if err != nil {
		return err
	}

	norm := normalize.New(dict)
	cats := catalog.NewCategorizer(rules)

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var total, correct int
	var wrong, missing []string

	in := bufio.NewScanner(f)
	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		raw, want, ok := strings.Cut(line, ";")
		if !ok {
			continue
		}
		total++

		canonical := norm.Name(raw).Canonical
		id, found := cats.Categorize(canonical)

		switch {
		case !found:
			missing = append(missing, fmt.Sprintf("%-44s -> %s", raw, canonical))
		case codes[id] != want:
			wrong = append(wrong, fmt.Sprintf("%-44s -> %s (ожидалось %s)", raw, codes[id], want))
		default:
			correct++
		}
	}

	fmt.Printf("верно %d из %d (%.1f%%), без категории %d, ошибок %d\n",
		correct, total, float64(correct)/float64(total)*100, len(missing), len(wrong))

	report("ошиблись категорией", wrong)
	report("без категории", missing)
	return in.Err()
}

func report(title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Printf("\n%s:\n", title)
	for _, s := range items {
		fmt.Println(" ", s)
	}
}

func categoryCodes(ctx context.Context, pool *pgxpool.Pool) (map[int64]string, error) {
	rows, err := pool.Query(ctx, `SELECT id, code FROM categories`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			return nil, err
		}
		out[id] = code
	}
	return out, rows.Err()
}
