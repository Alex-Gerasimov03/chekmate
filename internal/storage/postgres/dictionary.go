package postgres

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alex-Gerasimov03/chekmate/internal/normalize"
)

type Dictionaries struct{ pool *pgxpool.Pool }

func NewDictionaries(pool *pgxpool.Pool) *Dictionaries { return &Dictionaries{pool: pool} }

// Load собирает словарь целиком: он мал, меняется редко и дальше живёт в памяти.
func (d *Dictionaries) Load(ctx context.Context) (normalize.Dictionary, error) {
	dict := normalize.Dictionary{
		Abbrev:    map[string]string{},
		StopWords: map[string]bool{},
	}

	rows, err := d.pool.Query(ctx, `SELECT short, expansion FROM abbreviations`)
	if err != nil {
		return dict, fmt.Errorf("чтение сокращений: %w", err)
	}
	for rows.Next() {
		var short, full string
		if err := rows.Scan(&short, &full); err != nil {
			return dict, err
		}
		dict.Abbrev[short] = full
	}
	if err := rows.Err(); err != nil {
		return dict, err
	}

	words, err := d.pool.Query(ctx, `SELECT word FROM stop_words`)
	if err != nil {
		return dict, fmt.Errorf("чтение стоп-слов: %w", err)
	}
	for words.Next() {
		var w string
		if err := words.Scan(&w); err != nil {
			return dict, err
		}
		dict.StopWords[w] = true
	}
	if err := words.Err(); err != nil {
		return dict, err
	}

	dict.Version = version(dict)
	return dict, nil
}

// version — отпечаток содержимого словаря. По его изменению видно, что
// канонические ключи товаров пора пересчитать.
func version(d normalize.Dictionary) int {
	parts := make([]string, 0, len(d.Abbrev)+len(d.StopWords))
	for k, v := range d.Abbrev {
		parts = append(parts, k+"="+v)
	}
	for w := range d.StopWords {
		parts = append(parts, "!"+w)
	}
	sort.Strings(parts)

	h := fnv.New32a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
	}
	return int(h.Sum32() & 0x7fffffff)
}
