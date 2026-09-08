package qr

import (
	"image"
	"image/color"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func encodeQR(t *testing.T, text string, size int) image.Image {
	t.Helper()

	matrix, err := qrcode.NewQRCodeWriter().Encode(text, gozxing.BarcodeFormat_QR_CODE, size, size, nil)
	if err != nil {
		t.Fatal(err)
	}
	return matrix
}

func TestDecode(t *testing.T) {
	const raw = "t=20260906T1215&s=1543.20&fn=9960440300123456&i=12345&fp=1234567890&n=1"

	got, err := Decode(encodeQR(t, raw, 256))
	if err != nil {
		t.Fatal(err)
	}
	if got != raw {
		t.Errorf("декодировано %q, ожидалось %q", got, raw)
	}
}

// Снимок чека почти никогда не бывает контрастным: проверяем, что
// предобработка вытягивает блёклое изображение.
func TestDecodeLowContrast(t *testing.T) {
	const raw = "t=20260906T1215&s=99.90&fn=996&i=1&fp=2&n=1"
	src := encodeQR(t, raw, 256)

	b := src.Bounds()
	faded := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.GrayModel.Convert(src.At(x, y)).(color.Gray)
			faded.SetGray(x, y, color.Gray{Y: 100 + uint8(float64(c.Y)*0.4)})
		}
	}

	if _, err := Decode(faded); err != nil {
		t.Errorf("блёклый снимок не распознан: %v", err)
	}
}

func TestDecodeNoCode(t *testing.T) {
	blank := image.NewGray(image.Rect(0, 0, 64, 64))
	for i := range blank.Pix {
		blank.Pix[i] = 255
	}

	if _, err := Decode(blank); err != ErrNoQR {
		t.Errorf("ожидалась ErrNoQR, получено %v", err)
	}
}
