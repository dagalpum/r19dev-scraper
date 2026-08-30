package tui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/cache"
	"golang.org/x/image/draw"
)

// FetchAndRenderCover downloads or loads from disk cache, and converts to ANSI Truecolor half-block.
func FetchAndRenderCover(id, coverURL string, proto GraphicProtocol, targetWidth, targetHeight int) (string, error) {
	// 1. Check local persistent disk cache first
	if localBytes, found := cache.Default().GetImage(id); found && len(localBytes) > 0 {
		img, _, err := image.Decode(bytes.NewReader(localBytes))
		if err == nil {
			return ImageToANSI(img, targetWidth, targetHeight), nil
		}
	}

	if coverURL == "" {
		return "", fmt.Errorf("empty cover URL")
	}

	// 2. Fetch from remote HTTP
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodGet, coverURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://r18.dev/")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch image: status %d", resp.StatusCode)
	}

	rawBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 3. Persist original image to disk cache for future instant loads
	_ = cache.Default().SetImage(id, rawBytes)

	img, _, err := image.Decode(bytes.NewReader(rawBytes))
	if err != nil {
		return "", fmt.Errorf("image decode error: %w", err)
	}

	return ImageToANSI(img, targetWidth, targetHeight), nil
}

// ImageToANSI converts an image.Image into ANSI 24-bit half-block characters (▀)
// using high-quality Catmull-Rom bicubic resampling with aspect ratio preservation.
// targetHeight is in terminal character rows (each character row contains 2 vertical pixels).
func ImageToANSI(img image.Image, maxCols, maxRows int) string {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return ""
	}

	if maxCols <= 0 {
		maxCols = 38
	}
	if maxRows <= 0 {
		maxRows = 14
	}

	// Terminal monospace cells have an aspect ratio of roughly 1:2 (width:height).
	// Because half-block characters contain 2 vertical pixels per cell (top & bottom),
	// 1 character cell corresponds to a 1:1 square pixel area on screen!
	// Therefore, pixel width = maxCols, pixel height = maxRows * 2.
	maxPixelW := maxCols
	maxPixelH := maxRows * 2

	// Scale while strictly preserving aspect ratio
	pixelW := maxPixelW
	pixelH := pixelW * srcH / srcW
	if pixelH > maxPixelH {
		pixelH = maxPixelH
		pixelW = pixelH * srcW / srcH
	}
	if pixelW < 4 {
		pixelW = 4
	}
	if pixelH < 4 {
		pixelH = 4
	}

	// High-fidelity Catmull-Rom bicubic interpolation
	dst := image.NewRGBA(image.Rect(0, 0, pixelW, pixelH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	charRows := (pixelH + 1) / 2
	var b strings.Builder

	for row := 0; row < charRows; row++ {
		topY := row * 2
		botY := row*2 + 1

		for col := 0; col < pixelW; col++ {
			topC := dst.RGBAAt(col, topY)
			botC := color.RGBA{R: 0, G: 0, B: 0, A: 255}
			if botY < pixelH {
				botC = dst.RGBAAt(col, botY)
			}

			// \x1b[38;2;R;G;Bm = foreground (top pixel ▀), \x1b[48;2;R;G;Bm = background (bottom pixel)
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				topC.R, topC.G, topC.B,
				botC.R, botC.G, botC.B,
			)
		}
		b.WriteString("\x1b[0m\n")
	}

	return strings.TrimRight(b.String(), "\n")
}
