package jellyfin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

var (
	// Regex matching DMM sample image thumbnails like /cawb00006-15.jpg or /cawb00006-1.jpg
	dmmSampleRegex = regexp.MustCompile(`([a-z0-9]+)-([0-9]+)\.jpg$`)
	// Regex matching DMM cover thumbnails like /cawb00006ps.jpg
	dmmCoverRegex = regexp.MustCompile(`([a-z0-9]+)ps\.jpg$`)

	// Shared HTTP client with connection reuse & pooling for fast asset downloads
	defaultAssetHTTPClient = &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 30,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression: false,
		},
	}
)

// UpgradeDMMImageURL converts low-res DMM thumbnail URLs to their Full HD counterparts.
// E.g.:
//   - https://pics.dmm.co.jp/digital/video/cawb00006/cawb00006-15.jpg -> https://pics.dmm.co.jp/digital/video/cawb00006/cawb00006jp-15.jpg
//   - https://pics.dmm.co.jp/digital/video/cawb00006/cawb00006ps.jpg -> https://pics.dmm.co.jp/digital/video/cawb00006/cawb00006pl.jpg
func UpgradeDMMImageURL(imgURL string) string {
	imgURL = strings.TrimSpace(imgURL)
	if imgURL == "" {
		return ""
	}

	// If relative DMM path like "digital/video/..." or "mono/movie/...", prefix with DMM CDN
	if !strings.HasPrefix(imgURL, "http://") && !strings.HasPrefix(imgURL, "https://") {
		if strings.HasPrefix(imgURL, "digital/") || strings.HasPrefix(imgURL, "mono/") {
			imgURL = "https://pics.dmm.co.jp/" + imgURL
		}
	}
	if (strings.HasPrefix(imgURL, "https://pics.dmm.co.jp/") || strings.HasPrefix(imgURL, "http://pics.dmm.co.jp/")) &&
		!strings.HasSuffix(imgURL, ".jpg") && !strings.HasSuffix(imgURL, ".png") && !strings.HasSuffix(imgURL, ".webp") {
		imgURL += ".jpg"
	}

	// Upgrade sample screenshots: id-15.jpg -> idjp-15.jpg
	if dmmSampleRegex.MatchString(imgURL) && !strings.Contains(imgURL, "jp-") {
		return dmmSampleRegex.ReplaceAllString(imgURL, "${1}jp-${2}.jpg")
	}

	// Upgrade cover: idps.jpg -> idpl.jpg
	if dmmCoverRegex.MatchString(imgURL) {
		return dmmCoverRegex.ReplaceAllString(imgURL, "${1}pl.jpg")
	}

	return imgURL
}

// DownloadAsset downloads an image from imageURL and saves it to destPath.
func DownloadAsset(ctx context.Context, imageURL, destPath string) error {
	if imageURL == "" || destPath == "" {
		return fmt.Errorf("empty image URL or destination path")
	}

	upgradedURL := UpgradeDMMImageURL(imageURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upgradedURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", scraper.DefaultUA)
	req.Header.Set("Referer", "https://r18.dev/")

	resp, err := defaultAssetHTTPClient.Do(req)
	if err != nil {
		// Fallback to original URL if upgraded URL failed
		if upgradedURL != imageURL {
			reqOrig, oErr := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
			if oErr == nil {
				reqOrig.Header.Set("User-Agent", scraper.DefaultUA)
				resp, err = defaultAssetHTTPClient.Do(reqOrig)
			}
		}
		if err != nil {
			return fmt.Errorf("failed to download asset from %s: %w", imageURL, err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error %d downloading %s", resp.StatusCode, upgradedURL)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// ProgressReporter defines a callback function to report asset download progress.
type ProgressReporter func(step string, current, total int, message string)

// DownloadAllAssetsWithProgress downloads poster.jpg, fanart.jpg, and extrafanart/ sample screenshots for Jellyfin with live progress reporting.
func DownloadAllAssetsWithProgress(ctx context.Context, movie *scraper.Movie, movieDir string, reporter ProgressReporter) error {
	if movie == nil || movieDir == "" {
		return fmt.Errorf("invalid movie or target directory")
	}

	if err := os.MkdirAll(movieDir, 0o755); err != nil {
		return err
	}

	// 1. Download Poster / Cover (High-Res Jacket)
	posterURL := movie.CoverURL
	if posterURL == "" {
		posterURL = movie.PosterURL
	}
	if posterURL != "" {
		if reporter != nil {
			reporter("download_poster", 1, 2, "กำลังดาวน์โหลดภาพปกความละเอียดสูง (poster.jpg)...")
		}
		posterPath := filepath.Join(movieDir, "poster.jpg")
		_ = DownloadAsset(ctx, posterURL, posterPath)

		if reporter != nil {
			reporter("download_fanart", 2, 2, "กำลังดาวน์โหลดภาพ backdrop (fanart.jpg)...")
		}
		fanartPath := filepath.Join(movieDir, "fanart.jpg")
		_ = DownloadAsset(ctx, posterURL, fanartPath)
	}

	// 2. Download Sample Screenshots concurrently into extrafanart/
	totalScreenshots := len(movie.SampleScreenshots)
	if totalScreenshots > 0 {
		extraDir := filepath.Join(movieDir, "extrafanart")
		if err := os.MkdirAll(extraDir, 0o755); err == nil {
			sem := make(chan struct{}, 5)
			var wg sync.WaitGroup
			var downloadedCount int32

			for i, rawURL := range movie.SampleScreenshots {
				idx := i + 1
				url := rawURL
				sampleFile := filepath.Join(extraDir, fmt.Sprintf("fanart%d.jpg", idx))

				wg.Add(1)
				go func(idx int, u, dest string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					_ = DownloadAsset(ctx, u, dest)
					done := atomic.AddInt32(&downloadedCount, 1)
					if reporter != nil {
						reporter("download_screenshot", int(done), totalScreenshots, fmt.Sprintf("กำลังดาวน์โหลดภาพตัวอย่าง Screenshot (%d/%d)...", int(done), totalScreenshots))
					}
				}(idx, url, sampleFile)
			}
			wg.Wait()
		}
	}

	return nil
}

// DownloadAllAssets downloads poster.jpg, fanart.jpg, and extrafanart/ sample screenshots for Jellyfin.
func DownloadAllAssets(ctx context.Context, movie *scraper.Movie, movieDir string) error {
	return DownloadAllAssetsWithProgress(ctx, movie, movieDir, nil)
}
