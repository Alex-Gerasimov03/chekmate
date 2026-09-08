package bot

import (
	"sync"
	"time"
)

// limiter ограничивает частоту сообщений от одного пользователя.
type limiter struct {
	mu     sync.Mutex
	counts map[int64]*window
	limit  int
	period time.Duration
}

type window struct {
	count int
	since time.Time
}

func newLimiter(limit int, period time.Duration) *limiter {
	return &limiter{counts: map[int64]*window{}, limit: limit, period: period}
}

func (l *limiter) allow(userID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	w, ok := l.counts[userID]
	if !ok || now.Sub(w.since) > l.period {
		l.counts[userID] = &window{count: 1, since: now}
		l.evictLocked(now)
		return true
	}
	if w.count >= l.limit {
		return false
	}
	w.count++
	return true
}

func (l *limiter) evictLocked(now time.Time) {
	for id, w := range l.counts {
		if now.Sub(w.since) > l.period {
			delete(l.counts, id)
		}
	}
}
