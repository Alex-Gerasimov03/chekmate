package normalize

import (
	"encoding/csv"
	"os"
	"strconv"
	"testing"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	r.Comma = ';'
	r.Comment = '#'
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows[1:]
}

// Словарь для тестов берётся из testdata: алгоритм проверяется отдельно
// от наполнения, которое живёт в базе.
func testNormalizer(t *testing.T) *Normalizer {
	t.Helper()

	d := Dictionary{
		Abbrev:    map[string]string{},
		StopWords: map[string]bool{},
		Version:   1,
	}
	for _, row := range readCSV(t, "testdata/dict.csv") {
		d.Abbrev[row[0]] = row[1]
	}
	for _, row := range readCSV(t, "testdata/stop.csv") {
		d.StopWords[row[0]] = true
	}
	return New(d)
}

func TestName(t *testing.T) {
	n := testNormalizer(t)

	for _, row := range readCSV(t, "testdata/names.csv") {
		raw, wantCanonical, wantTitle, wantQty, wantFat := row[0], row[1], row[2], row[3], row[4]

		t.Run(raw, func(t *testing.T) {
			got := n.Name(raw)

			if got.Canonical != wantCanonical {
				t.Errorf("Canonical = %q, ожидалось %q", got.Canonical, wantCanonical)
			}
			if got.Title != wantTitle {
				t.Errorf("Title = %q, ожидалось %q", got.Title, wantTitle)
			}

			var want domain.Quantity
			if wantQty != "" {
				want = domain.MustParseQuantity(wantQty)
			}
			if got.Qty != want {
				t.Errorf("Qty = %v, ожидалось %v", got.Qty, want)
			}

			fat, err := strconv.Atoi(wantFat)
			if err != nil {
				t.Fatalf("некорректная жирность в testdata: %v", err)
			}
			if got.Fat != fat {
				t.Errorf("Fat = %d, ожидалось %d", got.Fat, fat)
			}
		})
	}
}

// Ради этого свойства нормализация и существует: один товар, записанный
// в разных магазинах по-разному, должен давать один ключ.
func TestNameVariantsCollapse(t *testing.T) {
	n := testNormalizer(t)

	groups := [][]string{
		{"МОЛОКО ПРОСТОКВ.3,2% 930МЛ", "Молоко Простоквашино 3.2 930г", "мол. простоквашино"},
		{"Масло слив. 82,5%", "МАСЛО СЛИВОЧНОЕ 82.5"},
		{"КАРТОФЕЛЬ МЫТЫЙ 1КГ", "картофель мытый вес"},
	}

	for _, g := range groups {
		want := n.Name(g[0]).Canonical
		for _, v := range g[1:] {
			if got := n.Name(v).Canonical; got != want {
				t.Errorf("%q дало %q, а %q дало %q", v, got, g[0], want)
			}
		}
	}
}

// Стеммер должен схлопывать словоформы без единой записи в словаре.
func TestNameCollapsesWordForms(t *testing.T) {
	n := testNormalizer(t)

	for _, g := range [][2]string{
		{"яблоки", "яблоко"},
		{"конфеты", "конфета"},
		{"сосиски молочные", "сосиска молочная"},
	} {
		if a, b := n.Name(g[0]).Canonical, n.Name(g[1]).Canonical; a != b {
			t.Errorf("%q и %q должны схлопываться: %q против %q", g[0], g[1], a, b)
		}
	}
}

func TestNameIdempotent(t *testing.T) {
	n := testNormalizer(t)

	for _, raw := range []string{"МОЛОКО ПРОСТОКВ.3,2% 930МЛ", "ГОВ.ВЫРЕЗКА ОХЛ. 1КГ"} {
		once := n.Name(raw).Canonical
		if twice := n.Name(once).Canonical; twice != once {
			t.Errorf("повторная нормализация %q изменила результат: %q -> %q", raw, once, twice)
		}
	}
}

func TestNameEmpty(t *testing.T) {
	n := testNormalizer(t)

	for _, raw := range []string{"", "   ", "123", "!!!"} {
		if got := n.Name(raw); got.Canonical != "" {
			t.Errorf("Name(%q).Canonical = %q, ожидалась пустая строка", raw, got.Canonical)
		}
	}
}
