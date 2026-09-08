package bot

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// pendingQuestion — заданный пользователю вопрос о принадлежности позиции.
type pendingQuestion struct {
	ReceiptID int64
	BudgetID  int64
	RawName   string
	ProductID int64 // предложенный товар
	Title     string
	created   time.Time
}

// pendingStore держит открытые вопросы в памяти.
type pendingStore struct {
	mu    sync.Mutex
	items map[string]pendingQuestion
	ttl   time.Duration
}

func newPendingStore(ttl time.Duration) *pendingStore {
	return &pendingStore{items: map[string]pendingQuestion{}, ttl: ttl}
}

func (s *pendingStore) put(q pendingQuestion) string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return ""
	}
	id := hex.EncodeToString(buf[:])

	q.created = time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
	s.items[id] = q
	return id
}

func (s *pendingStore) take(id string) (pendingQuestion, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	q, ok := s.items[id]
	if !ok || time.Since(q.created) > s.ttl {
		delete(s.items, id)
		return pendingQuestion{}, false
	}
	delete(s.items, id)
	return q, true
}

func (s *pendingStore) evictLocked() {
	for id, q := range s.items {
		if time.Since(q.created) > s.ttl {
			delete(s.items, id)
		}
	}
}
