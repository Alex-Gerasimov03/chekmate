package catalog

import (
	"context"
	"testing"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
	"github.com/Alex-Gerasimov03/chekmate/internal/normalize"
)

// fakeRepo заменяет базу: логика матчера не зависит от хранилища, и проверять
// её быстрее без контейнера. Работа самих запросов проверяется отдельно.
type fakeRepo struct {
	aliases  map[string]int64
	products map[int64]NewProduct
	similar  []Candidate

	aliasCalls int
	nextID     int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{aliases: map[string]int64{}, products: map[int64]NewProduct{}}
}

func (r *fakeRepo) AliasProduct(_ context.Context, raw string) (int64, bool, error) {
	r.aliasCalls++
	id, ok := r.aliases[raw]
	return id, ok, nil
}

func (r *fakeRepo) FindSimilar(context.Context, string, domain.Unit, int) ([]Candidate, error) {
	return r.similar, nil
}

func (r *fakeRepo) CreateProduct(_ context.Context, p NewProduct) (int64, error) {
	r.nextID++
	r.products[r.nextID] = p
	return r.nextID, nil
}

func (r *fakeRepo) SaveAlias(_ context.Context, raw string, productID int64, _ bool) error {
	r.aliases[raw] = productID
	return nil
}

func testMatcher(repo Repo) *Matcher {
	dict := normalize.Dictionary{
		Abbrev:    map[string]string{"мол": "молоко", "простокв": "простоквашино"},
		StopWords: map[string]bool{"вес": true},
		Version:   7,
	}
	return NewMatcher(repo, normalize.New(dict))
}

func TestMatchCreatesUnknownProduct(t *testing.T) {
	repo := newFakeRepo()
	m := testMatcher(repo)

	got, err := m.Match(context.Background(), "МОЛОКО ПРОСТОКВ.3,2% 930МЛ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeCreated {
		t.Fatalf("outcome = %d, ожидался OutcomeCreated", got.Outcome)
	}

	p := repo.products[got.ProductID]
	if p.Canonical != "молок простоквашин" {
		t.Errorf("канон %q", p.Canonical)
	}
	if p.Fat != 320 {
		t.Errorf("жирность %d, ожидалось 320", p.Fat)
	}
	if p.Unit != domain.UnitL || p.PackMilli != 930 {
		t.Errorf("упаковка %v %d, ожидалось l 930", p.Unit, p.PackMilli)
	}
	if p.Version != 7 {
		t.Errorf("версия словаря %d, ожидалась 7", p.Version)
	}
}

func TestMatchBindsConfidentCandidate(t *testing.T) {
	repo := newFakeRepo()
	repo.similar = []Candidate{{ProductID: 42, Title: "молоко простоквашино", Similarity: 0.83}}
	m := testMatcher(repo)

	got, err := m.Match(context.Background(), "МОЛ.ПРОСТОКВАШИНО 930")
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeMatched || got.ProductID != 42 {
		t.Fatalf("outcome = %d, товар = %d", got.Outcome, got.ProductID)
	}
	if repo.aliases["МОЛ.ПРОСТОКВАШИНО 930"] != 42 {
		t.Error("уверенное совпадение должно сохраняться алиасом")
	}
}

// Пограничное сходство не привязывается молча: пока пользователь не ответил,
// связь неизвестна, и записывать её нельзя.
func TestMatchAsksWhenUnsure(t *testing.T) {
	repo := newFakeRepo()
	repo.similar = []Candidate{{ProductID: 42, Title: "молоко простоквашино", Similarity: 0.45}}
	m := testMatcher(repo)

	got, err := m.Match(context.Background(), "молоко деревенское")
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeAmbiguous {
		t.Fatalf("outcome = %d, ожидался OutcomeAmbiguous", got.Outcome)
	}
	if got.Suggestion == nil || got.Suggestion.ProductID != 42 {
		t.Fatal("ожидалось предложение товара")
	}
	if len(repo.aliases) != 0 {
		t.Error("неподтверждённая связь не должна попадать в алиасы")
	}
}

// Ответ пользователя закрывает вопрос навсегда.
func TestConfirmRemembersChoice(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.similar = []Candidate{{ProductID: 42, Similarity: 0.45}}
	m := testMatcher(repo)

	if err := m.Confirm(ctx, "молоко деревенское", 42); err != nil {
		t.Fatal(err)
	}

	got, err := m.Match(ctx, "молоко деревенское")
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeAlias || got.ProductID != 42 {
		t.Errorf("outcome = %d, товар = %d", got.Outcome, got.ProductID)
	}
}

// Кэш существует ради того, чтобы повторные строки не ходили в базу.
func TestMatchUsesCache(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	m := testMatcher(repo)

	const raw = "ХЛЕБ БОРОДИНСКИЙ 400Г"
	if _, err := m.Match(ctx, raw); err != nil {
		t.Fatal(err)
	}
	before := repo.aliasCalls

	for i := 0; i < 5; i++ {
		if _, err := m.Match(ctx, raw); err != nil {
			t.Fatal(err)
		}
	}
	if repo.aliasCalls != before {
		t.Errorf("после прогрева кэша было %d обращений к базе, стало %d", before, repo.aliasCalls)
	}
}

func TestMatchRejectsGarbage(t *testing.T) {
	if _, err := testMatcher(newFakeRepo()).Match(context.Background(), "!!! 123"); err == nil {
		t.Error("из строки без названия товар создаваться не должен")
	}
}
