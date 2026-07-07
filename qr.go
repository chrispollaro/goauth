package main

import (
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/kbinani/screenshot"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func decodeQRImage(img image.Image) (string, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		return "", err
	}
	return res.GetText(), nil
}

func QRFromFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}
	return decodeQRImage(img)
}

// QRFromScreen captures every display and scans each for a QR code.
func QRFromScreen() (string, error) {
	for i := 0; i < screenshot.NumActiveDisplays(); i++ {
		img, err := screenshot.CaptureDisplay(i)
		if err != nil {
			continue
		}
		if text, err := decodeQRImage(img); err == nil {
			return text, nil
		}
	}
	return "", errors.New("no QR code found on any display")
}
