package bot

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// Telegram присылает фотографии в JPEG. Декодер регистрируется пустым
// импортом, который легко потерять при чистке — тест это ловит.
func TestJPEGDecoderRegistered(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 8, 8))
	src.SetGray(1, 1, color.Gray{Y: 255})

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, nil); err != nil {
		t.Fatal(err)
	}

	_, format, err := image.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("JPEG не декодируется: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("формат %q, ожидался jpeg", format)
	}
}
