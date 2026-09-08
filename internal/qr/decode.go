package qr

import (
	"errors"
	"image"
	"image/color"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// ErrNoQR означает, что на снимке не нашлось ни одного QR-кода.
var ErrNoQR = errors.New("на фотографии не найден QR-код")

var hints = map[gozxing.DecodeHintType]interface{}{
	gozxing.DecodeHintType_TRY_HARDER: true,
}

// Decode вытаскивает строку из QR-кода на снимке.
func Decode(img image.Image) (string, error) {
	reader := qrcode.NewQRCodeReader()

	for _, prepare := range []func(image.Image) image.Image{
		func(i image.Image) image.Image { return i },
		grayscale,
		contrast,
	} {
		bmp, err := gozxing.NewBinaryBitmapFromImage(prepare(img))
		if err != nil {
			continue
		}
		res, err := reader.Decode(bmp, hints)
		if err == nil {
			return res.GetText(), nil
		}
	}
	return "", ErrNoQR
}

func grayscale(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}
	return dst
}

// contrast растягивает яркость на весь диапазон: на снимках чеков она обычно
// сжата в светлой части, и порог бинаризации промахивается.
func contrast(src image.Image) image.Image {
	b := src.Bounds()
	gray := image.NewGray(b)
	lo, hi := uint8(255), uint8(0)

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.GrayModel.Convert(src.At(x, y)).(color.Gray)
			gray.SetGray(x, y, c)
			if c.Y < lo {
				lo = c.Y
			}
			if c.Y > hi {
				hi = c.Y
			}
		}
	}
	if hi <= lo {
		return gray
	}

	span := float64(hi - lo)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			v := float64(gray.GrayAt(x, y).Y-lo) / span * 255
			gray.SetGray(x, y, color.Gray{Y: uint8(v)})
		}
	}
	return gray
}
