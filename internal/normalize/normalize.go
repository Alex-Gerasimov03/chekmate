// Package normalize приводит названия товаров из чеков к каноническому виду,
// пригодному для сопоставления одинаковых товаров между магазинами.
package normalize

import (
	"regexp"
	"strings"

	"github.com/kljensen/snowball/russian"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

// Dictionary — данные, от которых зависит разбор. Живут в базе, а не в коде:
// пополнять их можно без пересборки, в том числе из бота.
type Dictionary struct {
	Abbrev    map[string]string
	StopWords map[string]bool
	Version   int
}

// Result — разобранное название товара.
type Result struct {
	Canonical string          // основы слов, ключ сопоставления
	Title     string          // читаемое название для показа пользователю
	Qty       domain.Quantity // нулевое, если объём в названии не указан
	Fat       int             // жирность в сотых долях процента, 0 если нет
}

type Normalizer struct{ dict Dictionary }

func New(d Dictionary) *Normalizer { return &Normalizer{dict: d} }

func (n *Normalizer) Version() int { return n.dict.Version }

const decimalMark = "~" // временно подменяет разделитель дробной части

var (
	fatRe     = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*%`)
	decimalRe = regexp.MustCompile(`(\d)[.,](\d)`)
	splitRe   = regexp.MustCompile(`(\d)([а-яa-z])|([а-яa-z])(\d)`)
	junkRe    = regexp.MustCompile(`[^0-9а-яa-z~]+`)
	numberRe  = regexp.MustCompile(`^\d+(?:\.\d+)?$`)
)

// units — единицы, которые могут стоять при числе в названии. Это часть
// алгоритма разбора, а не словарь: набор единиц не меняется.
var units = map[string]bool{
	"г": true, "гр": true, "кг": true, "мл": true, "л": true,
	"g": true, "kg": true, "ml": true, "l": true, "шт": true,
}

// Name разбирает название так, как оно напечатано в чеке.
func (n *Normalizer) Name(raw string) Result {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "ё", "е")

	var res Result
	if m := fatRe.FindStringSubmatch(s); m != nil {
		res.Fat = parseFat(m[1])
		s = strings.Replace(s, m[0], " ", 1)
	}

	// Сокращения со слэшем раскрываются до того, как слэш станет разделителем.
	for short, full := range n.dict.Abbrev {
		if strings.Contains(short, "/") {
			s = strings.ReplaceAll(s, short, " "+full+" ")
		}
	}

	s = decimalRe.ReplaceAllString(s, "$1"+decimalMark+"$2")
	s = splitRe.ReplaceAllString(s, "$1$3 $2$4")
	s = junkRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, decimalMark, ".")

	tokens := strings.Fields(s)
	words := make([]string, 0, len(tokens))

	for i := 0; i < len(tokens); i++ {
		t := tokens[i]

		if numberRe.MatchString(t) {
			if i+1 < len(tokens) && units[tokens[i+1]] {
				if q, err := domain.ParseQuantity(t + tokens[i+1]); err == nil && res.Qty.IsZero() {
					res.Qty = q
				}
				i++
				continue
			}
			// Дробное число без единицы — почти всегда жирность,
			// напечатанная без знака процента.
			if res.Fat == 0 && strings.Contains(t, ".") {
				res.Fat = parseFat(t)
			}
			continue
		}

		if full, ok := n.dict.Abbrev[t]; ok {
			words = append(words, strings.Fields(full)...)
			continue
		}
		if n.dict.StopWords[t] || units[t] {
			continue
		}
		words = append(words, t)
	}

	words = dedup(words)
	res.Title = strings.Join(words, " ")

	stems := make([]string, len(words))
	for i, w := range words {
		stems[i] = Stem(w)
	}
	res.Canonical = strings.Join(dedup(stems), " ")

	return res
}

// Stem доводит основу до неподвижной точки: snowball не идемпотентен, а
// нестабильный ключ расщепил бы один товар на несколько записей.
func Stem(w string) string {
	for i := 0; i < 4; i++ {
		s := russian.Stem(w, false)
		if s == w {
			break
		}
		w = s
	}
	return w
}

// parseFat переводит "3,2" в 320 сотых долей процента.
func parseFat(s string) int {
	whole, frac, _ := strings.Cut(strings.Replace(s, ",", ".", 1), ".")
	n := 0
	for _, r := range whole {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	n *= 100
	for i, r := range frac {
		if i >= 2 || r < '0' || r > '9' {
			break
		}
		if i == 0 {
			n += int(r-'0') * 10
		} else {
			n += int(r - '0')
		}
	}
	return n
}

// dedup убирает повторы, возникающие после раскрытия сокращений:
// "мол. молоко" превращается в одно слово.
func dedup(words []string) []string {
	seen := make(map[string]bool, len(words))
	out := make([]string, 0, len(words))
	for _, w := range words {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}
