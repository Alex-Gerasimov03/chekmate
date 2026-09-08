package postgres

import (
	"context"
	"testing"

	"github.com/Alex-Gerasimov03/chekmate/internal/catalog"
	"github.com/Alex-Gerasimov03/chekmate/internal/normalize"
)

// Сквозная проверка сида: строка из чека проходит нормализацию и попадает
// в категорию. Проверяются именно данные миграции, а не логика.
func TestSeedRulesCategorizeRealNames(t *testing.T) {
	ctx := context.Background()

	dict, err := NewDictionaries(pool).Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	norm := normalize.New(dict)

	rules, err := NewRules(pool).Load(ctx, newBudget(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) < 50 {
		t.Fatalf("правил загружено %d, ожидалось больше 50", len(rules))
	}
	cats := catalog.NewCategorizer(rules)

	codes := map[string]int64{}
	rows, err := pool.Query(ctx, `SELECT code, id FROM categories`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var code string
		var id int64
		if err := rows.Scan(&code, &id); err != nil {
			t.Fatal(err)
		}
		codes[code] = id
	}
	rows.Close()

	tests := []struct {
		raw  string
		want string
	}{
		{"МОЛОКО ПРОСТОКВ.3,2% 930МЛ", "dairy"},
		{"Масло слив. Традиционное 82,5% 180г", "dairy"},
		{"КОЛБАСА ДОКТОРСКАЯ 400Г", "meat"},
		{"ГОВ.ВЫРЕЗКА ОХЛ. 1КГ", "meat"},
		{"ХЛЕБ БОРОДИНСКИЙ 400Г", "bakery"},
		{"КАРТОФЕЛЬ МЫТЫЙ 1КГ", "vegetables"},
		{"Виноград кишмиш 500г", "vegetables"},
		{"Вино красное сухое 0,75л", "alcohol"},
		{"ПИВО СВЕТЛОЕ 0.5Л", "alcohol"},
		{"Чай Гринфилд 100 пак", "coffee_tea"},
		{"Кофе Jacobs Monarch 230г", "coffee_tea"},
		{"Корм для кошек мясной 85г", "pets"},
		{"Подгузники Pampers 62шт", "kids"},
		{"Зубная паста Colgate 100мл", "hygiene"},
		{"Шок.Аленка 90г", "sweets"},
		{"Вода питьевая 1,5л", "drinks"},

		// Строки из настоящих чеков: на них словарь и дорабатывался.
		{"PURINA ONE Корм сух д/котят курица/цел з", "pets"},
		{"TITBIT Лакомство д/кошек колбаски сливоч", "pets"},
		{"SNAQ FABRIQ Батончик глазир Арахис/карамель 5", "sweets"},
		{"COOL RULE Глина д/укл волос текстурир св", "hygiene"},
		{"МАГНИТ Косметик Пакет большой (Россия):100/17", "goods"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			canonical := norm.Name(tt.raw).Canonical

			got, ok := cats.Categorize(canonical)
			if !ok {
				t.Fatalf("категория не определена для %q", canonical)
			}
			if want := codes[tt.want]; got != want {
				var actual string
				_ = pool.QueryRow(ctx, `SELECT code FROM categories WHERE id = $1`, got).Scan(&actual)
				t.Errorf("%q -> %s, ожидалось %s", canonical, actual, tt.want)
			}
		})
	}
}

func TestRulesPersonalOverridesSeed(t *testing.T) {
	ctx := context.Background()
	budget := newBudget(t)
	repo := NewRules(pool)

	var grocery int64
	if err := pool.QueryRow(ctx, `SELECT id FROM categories WHERE code = 'grocery'`).Scan(&grocery); err != nil {
		t.Fatal(err)
	}
	if err := repo.Add(ctx, budget, grocery, "молоко"); err != nil {
		t.Fatal(err)
	}

	rules, err := repo.Load(ctx, budget)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := catalog.NewCategorizer(rules).Categorize("молок деревенск"); got != grocery {
		t.Errorf("личное правило не сработало: категория %d, ожидалась %d", got, grocery)
	}

	// Чужой бюджет не должен видеть это правило.
	otherRules, err := repo.Load(ctx, newBudget(t))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := catalog.NewCategorizer(otherRules).Categorize("молок деревенск"); got == grocery {
		t.Error("личное правило протекло в чужой бюджет")
	}
}
