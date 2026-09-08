package catalog

import "testing"

const (
	dairy = int64(iota + 1)
	meat
	pets
	grocery
	vegetables
	alcohol
	tea
	hygiene
)

func testCategorizer() *Categorizer {
	return NewCategorizer([]Rule{
		{Word: "молоко", CategoryID: dairy},
		{Word: "молочный", CategoryID: dairy},
		{Word: "масло", CategoryID: grocery},
		{Word: "масло сливочное", CategoryID: dairy, Priority: 10},
		{Word: "мясо", CategoryID: meat},
		{Word: "колбаса", CategoryID: meat},
		{Word: "корм", CategoryID: pets, Priority: 20},
		{Word: "яблоки", CategoryID: vegetables},
		{Word: "виноград", CategoryID: vegetables},
		{Word: "вино", CategoryID: alcohol, Priority: 20},
		{Word: "ром", CategoryID: alcohol, Priority: 20},
		{Word: "чай", CategoryID: tea},
		{Word: "кола", CategoryID: grocery},
		{Word: "зубная паста", CategoryID: hygiene, Priority: 10},
	})
}

func TestCategorize(t *testing.T) {
	c := testCategorizer()

	tests := []struct {
		canonical string
		want      int64
	}{
		{"молок простоквашин", dairy},
		{"молочн коктейл", dairy},
		{"масл подсолнечн", grocery},
		{"масл сливочн традицион", dairy},
		{"яблок голден", vegetables},
	}

	for _, tt := range tests {
		t.Run(tt.canonical, func(t *testing.T) {
			got, ok := c.Categorize(tt.canonical)
			if !ok {
				t.Fatal("категория не определена")
			}
			if got != tt.want {
				t.Errorf("категория %d, ожидалась %d", got, tt.want)
			}
		})
	}
}

// Короткое правило не должно цепляться к длинному слову: основа "вино" —
// это "вин", и при сравнении по префиксу она перехватывала бы "виноград".
func TestCategorizeDoesNotMatchByPrefix(t *testing.T) {
	c := testCategorizer()

	tests := []struct {
		canonical string
		want      int64
		note      string
	}{
		{"виноград кишмиш", vegetables, "виноград — не вино"},
		{"ромашков ча", tea, "ромашковый чай — не ром"},
		{"колбас докторск", meat, "колбаса — не кола"},
	}

	for _, tt := range tests {
		t.Run(tt.note, func(t *testing.T) {
			got, _ := c.Categorize(tt.canonical)
			if got != tt.want {
				t.Errorf("категория %d, ожидалась %d", got, tt.want)
			}
		})
	}
}

// Корм для кота содержит "мясо", но это товар для животных.
func TestCategorizeSpecificRuleWins(t *testing.T) {
	if got, _ := testCategorizer().Categorize("корм мяс кошк"); got != pets {
		t.Errorf("категория %d, ожидалась %d", got, pets)
	}
}

// Правило из нескольких слов важнее одиночного, даже с высоким приоритетом.
func TestCategorizeMultiWordRuleWins(t *testing.T) {
	c := NewCategorizer([]Rule{
		{Word: "паста", CategoryID: grocery, Priority: 20},
		{Word: "зубная паста", CategoryID: hygiene},
	})

	if got, _ := c.Categorize("зубн паст колгейт"); got != hygiene {
		t.Errorf("категория %d, ожидалась %d", got, hygiene)
	}
}

// Личное правило пользователя важнее общего.
func TestCategorizePersonalRuleWins(t *testing.T) {
	c := NewCategorizer([]Rule{
		{Word: "молоко", CategoryID: dairy},
		{Word: "молоко", CategoryID: grocery, Personal: true},
	})

	if got, _ := c.Categorize("молок деревенск"); got != grocery {
		t.Errorf("категория %d, ожидалась %d", got, grocery)
	}
}

func TestCategorizeUnknown(t *testing.T) {
	c := testCategorizer()

	for _, s := range []string{"", "неведом штук"} {
		if _, ok := c.Categorize(s); ok {
			t.Errorf("для %q категория не должна определяться", s)
		}
	}
}
