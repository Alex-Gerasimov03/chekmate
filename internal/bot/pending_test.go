package bot

import (
	"testing"
	"time"
)

func TestPendingStoreRoundTrip(t *testing.T) {
	s := newPendingStore(time.Minute)

	id := s.put(pendingQuestion{ReceiptID: 7, RawName: "МОЛОКО", ProductID: 42})
	if id == "" {
		t.Fatal("идентификатор не выдан")
	}

	got, ok := s.take(id)
	if !ok {
		t.Fatal("вопрос не найден")
	}
	if got.ReceiptID != 7 || got.ProductID != 42 {
		t.Errorf("вернулись другие данные: %+v", got)
	}
}

// Повторное нажатие той же кнопки не должно срабатывать дважды.
func TestPendingStoreConsumesOnce(t *testing.T) {
	s := newPendingStore(time.Minute)
	id := s.put(pendingQuestion{ReceiptID: 1})

	if _, ok := s.take(id); !ok {
		t.Fatal("первый ответ должен приниматься")
	}
	if _, ok := s.take(id); ok {
		t.Error("второй ответ на тот же вопрос принимать нельзя")
	}
}

func TestPendingStoreExpires(t *testing.T) {
	s := newPendingStore(time.Nanosecond)
	id := s.put(pendingQuestion{ReceiptID: 1})
	time.Sleep(time.Millisecond)

	if _, ok := s.take(id); ok {
		t.Error("просроченный вопрос не должен приниматься")
	}
}

func TestPendingStoreUnknownID(t *testing.T) {
	if _, ok := newPendingStore(time.Minute).take("нет такого"); ok {
		t.Error("неизвестный идентификатор не должен приниматься")
	}
}
