package qr

import (
	"image"
	"image/color"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

const benchRaw = "t=20260906T1215&s=1543.20&fn=9960440300123456&i=12345&fp=1234567890&n=1"

func benchImage(b *testing.B, size int) image.Image {
	b.Helper()
	m, err := qrcode.NewQRCodeWriter().Encode(benchRaw, gozxing.BarcodeFormat_QR_CODE, size, size, nil)
	if err != nil {
		b.Fatal(err)
	}
	return m
}

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Parse(benchRaw, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// Снимок из Telegram — около тысячи пикселей по стороне.
func BenchmarkDecode(b *testing.B) {
	img := benchImage(b, 1024)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := Decode(img); err != nil {
			b.Fatal(err)
		}
	}
}

// Худший случай: кода на снимке нет, и все три прохода отрабатывают впустую.
func BenchmarkDecodeNoCode(b *testing.B) {
	blank := image.NewGray(image.Rect(0, 0, 1024, 1024))
	for i := range blank.Pix {
		blank.Pix[i] = 255
	}
	blank.SetGray(10, 10, color.Gray{Y: 0})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Decode(blank); err == nil {
			b.Fatal("ожидалась ошибка")
		}
	}
}
