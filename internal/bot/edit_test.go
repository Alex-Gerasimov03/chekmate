package bot

import (
	"errors"
	"testing"
)

// Нажатие на кнопку уже выбранного периода — не сбой, и пользователю
// сообщать об этом не о чем.
func TestIsNotModified(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{errors.New("telegram: Bad Request: message is not modified (400)"), true},
		{errors.New("telegram: Bad Request: message to edit not found (400)"), false},
		{errors.New("dial tcp: i/o timeout"), false},
		{nil, false},
	}

	for _, tt := range tests {
		if got := isNotModified(tt.err); got != tt.want {
			t.Errorf("isNotModified(%v) = %v, ожидалось %v", tt.err, got, tt.want)
		}
	}
}
