package catalog

import (
	"fmt"
	"testing"
)

// benchRules повторяет объём рабочего словаря: несколько сотен правил.
func benchRules() []Rule {
	rules := []Rule{
		{Word: "молоко", CategoryID: 1, Priority: 20},
		{Word: "масло сливочное", CategoryID: 1, Priority: 40},
		{Word: "шницель", CategoryID: 2, Priority: 40},
		{Word: "конфеты", CategoryID: 3, Priority: 40},
		{Word: "птичье молоко", CategoryID: 3, Priority: 45},
	}
	for i := 0; i < 700; i++ {
		rules = append(rules, Rule{Word: fmt.Sprintf("товар%d", i), CategoryID: 9, Priority: 20})
	}
	return rules
}

func BenchmarkNewCategorizer(b *testing.B) {
	rules := benchRules()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = NewCategorizer(rules)
	}
}

func BenchmarkCategorize(b *testing.B) {
	c := NewCategorizer(benchRules())
	names := []string{
		"молок простоквашин",
		"благояр шницел венск сыр охлажд",
		"конфет птич молок сливочн ван",
		"неведом товар без правил",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Categorize(names[i%len(names)])
	}
}
