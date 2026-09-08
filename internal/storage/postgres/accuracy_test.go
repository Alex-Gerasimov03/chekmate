package postgres

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alex-Gerasimov03/chekmate/internal/catalog"
	"github.com/Alex-Gerasimov03/chekmate/internal/normalize"
)

// minAccuracy — порог, ниже которого правка словаря считается деградацией.
// Он ниже достигнутого, чтобы тест не падал от единичной перестановки, но
// ловил заметный откат.
const minAccuracy = 0.95

// Точность категоризации меряется на размеченных строках чеков. Наборы лежат
// в testdata и пополняются, когда встречается товар, который словарь не знает.
func TestCategorizationAccuracy(t *testing.T) {
	ctx := context.Background()

	dict, err := NewDictionaries(pool).Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := NewRules(pool).Load(ctx, newBudget(t))
	if err != nil {
		t.Fatal(err)
	}

	norm := normalize.New(dict)
	cats := catalog.NewCategorizer(rules)
	codes := categoryCodes(t)

	for _, name := range []string{
		"receipt_lines.csv",
		"receipt_lines_holdout.csv",
		"receipt_lines_holdout2.csv",
		"receipt_lines_real.csv",
	} {
		t.Run(name, func(t *testing.T) {
			total, correct := 0, 0

			for _, row := range labelledLines(t, name) {
				total++
				canonical := norm.Name(row.raw).Canonical

				id, ok := cats.Categorize(canonical)
				switch {
				case !ok:
					t.Logf("без категории: %-44s -> %s", row.raw, canonical)
				case codes[id] != row.want:
					t.Logf("ошибка: %-44s -> %s (ожидалось %s)", row.raw, codes[id], row.want)
				default:
					correct++
				}
			}

			if total == 0 {
				t.Fatal("набор пуст")
			}
			accuracy := float64(correct) / float64(total)
			t.Logf("точность %.1f%% (%d из %d)", accuracy*100, correct, total)

			if accuracy < minAccuracy {
				t.Errorf("точность %.1f%% ниже порога %.0f%%", accuracy*100, minAccuracy*100)
			}
		})
	}
}

type labelledLine struct{ raw, want string }

func labelledLines(t *testing.T, name string) []labelledLine {
	t.Helper()

	f, err := os.Open(filepath.Join("..", "..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var out []labelledLine
	in := bufio.NewScanner(f)
	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if raw, want, ok := strings.Cut(line, ";"); ok {
			out = append(out, labelledLine{raw: raw, want: want})
		}
	}
	if err := in.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func categoryCodes(t *testing.T) map[int64]string {
	t.Helper()

	rows, err := pool.Query(context.Background(), `SELECT id, code FROM categories`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			t.Fatal(err)
		}
		out[id] = code
	}
	return out
}
