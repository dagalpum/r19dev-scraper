package tui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"strings"
)

// GraphicProtocol represents terminal image graphics capability.
type GraphicProtocol string

const (
	ProtocolAuto      GraphicProtocol = "auto"
	ProtocolHalfBlock GraphicProtocol = "halfblock" // 24-bit Truecolor Half-Block (TUI-safe default)
	ProtocolKitty     GraphicProtocol = "kitty"     // Kitty Graphics Protocol (with q=2 quiet mode)
	ProtocolITerm2    GraphicProtocol = "iterm2"    // iTerm2 Inline Protocol
	ProtocolSixel     GraphicProtocol = "sixel"     // Sixel Protocol
)

// DetectTerminalProtocol returns the safest, highest-fidelity protocol for TUI layouts.
func DetectTerminalProtocol() GraphicProtocol {
	return ProtocolHalfBlock
}

// EncodeImageToProtocol converts an image to the chosen graphics protocol escape sequence.
// In Bubble Tea / Lipgloss layouts, 24-bit Truecolor Half-Block (▀) is the safest and highest-fidelity
// format because it integrates natively with terminal cells without breaking box borders or leaking escape sequences.
func EncodeImageToProtocol(img image.Image, rawBytes []byte, proto GraphicProtocol, widthCols, heightRows int) (string, error) {
	return ImageToANSI(img, widthCols, heightRows), nil
}

// EncodeITerm2 wraps image bytes in iTerm2's inline image escape protocol.
func EncodeITerm2(imgBytes []byte, widthCols, heightRows int) string {
	b64 := base64.StdEncoding.EncodeToString(imgBytes)
	return fmt.Sprintf("\x1b]1337;File=inline=1;width=%d;height=%d;preserveAspectRatio=1:%s\x07", widthCols, heightRows, b64)
}

// EncodeKitty wraps image bytes into Kitty Graphics Protocol chunks with q=2 (quiet mode to prevent stdin echo).
func EncodeKitty(pngBytes []byte, widthCols, heightRows int) string {
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	chunkSize := 4096
	var b strings.Builder

	// q=2 is critical: it tells Kitty NOT to send responses back to stdin (which corrupts Bubble Tea key handling)
	for i := 0; i < len(b64); i += chunkSize {
		end := i + chunkSize
		m := 1
		if end >= len(b64) {
			end = len(b64)
			m = 0
		}
		chunk := b64[i:end]
		if i == 0 {
			fmt.Fprintf(&b, "\x1b_Ga=T,f=100,q=2,c=%d,r=%d,m=%d;%s\x1b\\", widthCols, heightRows, m, chunk)
		} else {
			fmt.Fprintf(&b, "\x1b_Gq=2,m=%d;%s\x1b\\", m, chunk)
		}
	}
	return b.String()
}

// EncodeSixel converts image.Image to a Sixel graphic stream with color quantization.
func EncodeSixel(img image.Image, targetWidth, targetHeight int) (string, error) {
	if targetWidth <= 0 {
		targetWidth = 160
	}
	if targetHeight <= 0 {
		targetHeight = 160
	}

	resized := resizeImageSimple(img, targetWidth, targetHeight)
	bounds := resized.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	var b strings.Builder
	b.WriteString("\x1bPq\"1;1") // Sixel start introducer

	palette := []color.NRGBA{
		{0, 0, 0, 255},       // 0: Black
		{255, 0, 0, 255},     // 1: Red
		{0, 255, 0, 255},     // 2: Green
		{255, 255, 0, 255},   // 3: Yellow
		{0, 0, 255, 255},     // 4: Blue
		{255, 0, 255, 255},   // 5: Magenta
		{0, 255, 255, 255},   // 6: Cyan
		{255, 255, 255, 255}, // 7: White
		{128, 128, 128, 255}, // 8: Gray
		{128, 0, 0, 255},     // 9: Dark Red
		{0, 128, 0, 255},     // 10: Dark Green
		{128, 128, 0, 255},   // 11: Olive
		{0, 0, 128, 255},     // 12: Navy
		{128, 0, 128, 255},   // 13: Purple
		{0, 128, 128, 255},   // 14: Teal
		{192, 192, 192, 255}, // 15: Silver
	}

	for idx, col := range palette {
		rPct := int(col.R) * 100 / 255
		gPct := int(col.G) * 100 / 255
		bPct := int(col.B) * 100 / 255
		fmt.Fprintf(&b, "#%d;2;%d;%d;%d", idx, rPct, gPct, bPct)
	}

	for y := 0; y < h; y += 6 {
		for pIdx := range palette {
			hasPixelForColor := false
			var rowPattern strings.Builder

			for x := 0; x < w; x++ {
				var sixelVal byte
				for bit := 0; bit < 6; bit++ {
					py := y + bit
					if py < h {
						c := color.NRGBAModel.Convert(resized.At(bounds.Min.X+x, bounds.Min.Y+py)).(color.NRGBA)
						if closestColorIndex(c, palette) == pIdx {
							sixelVal |= (1 << bit)
							hasPixelForColor = true
						}
					}
				}
				rowPattern.WriteByte(sixelVal + 63)
			}

			if hasPixelForColor {
				fmt.Fprintf(&b, "#%d%s$", pIdx, rowPattern.String())
			}
		}
		b.WriteString("-")
	}

	b.WriteString("\x1b\\")
	return b.String(), nil
}

func closestColorIndex(c color.NRGBA, pal []color.NRGBA) int {
	bestIdx := 0
	minDist := 1000000
	for i, p := range pal {
		dr := int(c.R) - int(p.R)
		dg := int(c.G) - int(p.G)
		db := int(c.B) - int(p.B)
		dist := dr*dr + dg*dg + db*db
		if dist < minDist {
			minDist = dist
			bestIdx = i
		}
	}
	return bestIdx
}

func ensurePNGBytes(img image.Image, rawBytes []byte) ([]byte, error) {
	if len(rawBytes) > 8 && bytes.Equal(rawBytes[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return rawBytes, nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func resizeImageSimple(src image.Image, targetWidth, targetHeight int) image.Image {
	dst := image.NewNRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			srcX := x * srcW / targetWidth
			srcY := y * srcH / targetHeight
			dst.Set(x, y, src.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}
	return dst
}
