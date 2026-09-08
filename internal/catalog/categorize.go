// Package catalog сопоставляет строки чека с товарами и категориями.
package catalog

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Alex-Gerasimov03/chekmate/internal/normalize"
)

// Rule связывает слово с категорией. Personal означает правило, заданное
// пользователем: оно перебивает общие.
type Rule struct {
	Word       string
	CategoryID int64
	Personal   bool
	Priority   int
}

type rule struct {
	stems      []string
	categoryID int64
	personal   bool
	priority   int
}

type Categorizer struct{ rules []rule }

// NewCategorizer приводит правила к основам и упорядочивает их: личные выше
// общих, более длинное правило выше короткого, и лишь затем — приоритет.
func NewCategorizer(rules []Rule) *Categorizer {
	prepared := make([]rule, 0, len(rules))
	for _, r := range rules {
		words := strings.Fields(strings.ToLower(r.Word))
		if len(words) == 0 {
			continue
		}
		stems := make([]string, len(words))
		for i, w := range words {
			stems[i] = normalize.Stem(strings.ReplaceAll(w, "ё", "е"))
		}
		prepared = append(prepared, rule{
			stems: stems, categoryID: r.CategoryID,
			personal: r.Personal, priority: r.Priority,
		})
	}

	sort.SliceStable(prepared, func(i, j int) bool {
		a, b := prepared[i], prepared[j]
		if a.personal != b.personal {
			return a.personal
		}
		if len(a.stems) != len(b.stems) {
			return len(a.stems) > len(b.stems)
		}
		if a.priority != b.priority {
			return a.priority > b.priority
		}
		return false
	})
	return &Categorizer{rules: prepared}
}

// minPrefixStem — с какой длины (в буквах, не байтах) основу правила можно
// искать по префиксу.
const minPrefixStem = 5

// Categorize возвращает категорию для канонического названия.
func (c *Categorizer) Categorize(canonical string) (int64, bool) {
	if canonical == "" {
		return 0, false
	}

	stems := make(map[string]bool)
	for _, s := range strings.Fields(canonical) {
		stems[s] = true
	}

	for _, r := range c.rules {
		if matchesAll(stems, r.stems) {
			return r.categoryID, true
		}
	}
	return 0, false
}

// matchesAll требует присутствия всех слов правила; порядок не важен,
// в чеках название часто переставлено.
func matchesAll(stems map[string]bool, need []string) bool {
	for _, s := range need {
		if !matchStem(stems, s) {
			return false
		}
	}
	return true
}

func matchStem(stems map[string]bool, want string) bool {
	if stems[want] {
		return true
	}
	if utf8.RuneCountInString(want) < minPrefixStem {
		return false
	}
	// Только в одну сторону: обратное сравнение делает "лимон" частью
	// "лимонада". Разные основы одного слова заводятся в словаре отдельно.
	for s := range stems {
		if strings.HasPrefix(s, want) {
			return true
		}
	}
	return false
}
