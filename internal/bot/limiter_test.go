package bot

import (
	"testing"
	"time"
)

func TestLimiterAllowsWithinLimit(t *testing.T) {
	l := newLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !l.allow(1) {
			t.Fatalf("сообщение %d должно проходить", i+1)
		}
	}
	if l.allow(1) {
		t.Error("четвёртое сообщение должно отсекаться")
	}
}

func TestLimiterIsPerUser(t *testing.T) {
	l := newLimiter(1, time.Minute)

	if !l.allow(1) || !l.allow(2) {
		t.Error("ограничение одного пользователя не должно задевать другого")
	}
}

func TestLimiterResetsAfterPeriod(t *testing.T) {
	l := newLimiter(1, 10*time.Millisecond)

	l.allow(1)
	time.Sleep(20 * time.Millisecond)

	if !l.allow(1) {
		t.Error("после окончания окна счётчик должен обнуляться")
	}
}
