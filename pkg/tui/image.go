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
)

// FetchAndRenderCover downloads an image from coverURL and converts it using the specified graphics protocol.
func FetchAndRenderCover(coverURL string, proto GraphicProtocol, targetWidth, targetHeight int) (string, error) {
	if coverURL == "" {
		return "", fmt.Errorf("empty cover URL")
	}

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

	img, _, err := image.Decode(bytes.NewReader(rawBytes))
	if err != nil {
		return "", fmt.Errorf("image decode error: %w", err)
	}

	return EncodeImageToProtocol(img, rawBytes, proto, targetWidth, targetHeight)
}

// ImageToANSI converts an image.Image into ANSI 24-bit half-block characters (▀).
// targetHeight is in terminal character rows (each row contains 2 vertical pixels).
func ImageToANSI(img image.Image, targetWidth, targetHeight int) string {
	bounds := img.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()

	if targetWidth <= 0 {
		targetWidth = 26
	}
	if targetHeight <= 0 {
		targetHeight = 13
	}

	pixelHeight := targetHeight * 2
	var b strings.Builder

	for row := 0; row < targetHeight; row++ {
		topY := row * 2
		botY := row*2 + 1

		for col := 0; col < targetWidth; col++ {
			srcX := col * srcWidth / targetWidth
			srcTopY := topY * srcHeight / pixelHeight
			srcBotY := botY * srcHeight / pixelHeight

			topColor := color.NRGBAModel.Convert(img.At(bounds.Min.X+srcX, bounds.Min.Y+srcTopY)).(color.NRGBA)
			botColor := color.NRGBAModel.Convert(img.At(bounds.Min.X+srcX, bounds.Min.Y+srcBotY)).(color.NRGBA)

			// \x1b[38;2;R;G;Bm = foreground (top pixel), \x1b[48;2;R;G;Bm = background (bottom pixel), ▀ = half block
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				topColor.R, topColor.G, topColor.B,
				botColor.R, botColor.G, botColor.B,
			)
		}
		b.WriteString("\x1b[0m\n")
	}

	return strings.TrimRight(b.String(), "\n")
}
