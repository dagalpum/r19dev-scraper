package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

// Cache manages persistent local storage for metadata JSON files and raw image assets.
type Cache struct {
	rootDir     string
	metadataDir string
	imagesDir   string
	mu          sync.RWMutex
}

var (
	defaultInstance *Cache
	once            sync.Once
)

// Default returns the singleton global cache instance located in ~/.cache/r19dev.
func Default() *Cache {
	once.Do(func() {
		baseDir, err := os.UserCacheDir()
		if err != nil || baseDir == "" {
			home, hErr := os.UserHomeDir()
			if hErr == nil {
				baseDir = filepath.Join(home, ".cache")
			} else {
				baseDir = "."
			}
		}
		c, err := New(filepath.Join(baseDir, "r19dev"))
		if err != nil {
			// Fallback to local directory if user cache is unavailable
			c, _ = New(".cache_r19dev")
		}
		defaultInstance = c
	})
	return defaultInstance
}

// New creates a new Cache instance with specified root directory.
func New(rootDir string) (*Cache, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	metaDir := filepath.Join(absRoot, "metadata")
	imgDir := filepath.Join(absRoot, "images")

	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create metadata cache dir: %w", err)
	}
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create images cache dir: %w", err)
	}

	return &Cache{
		rootDir:     absRoot,
		metadataDir: metaDir,
		imagesDir:   imgDir,
	}, nil
}

// RootDir returns the root cache directory path.
func (c *Cache) RootDir() string {
	return c.rootDir
}

// GetMovie retrieves a cached Movie metadata record by combined ID or standard ID.
func (c *Cache) GetMovie(id string) (*scraper.Movie, bool) {
	if c == nil {
		return nil, false
	}
	combinedID := scraper.NormalizeToCombinedID(id)
	if combinedID == "" {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	filePath := filepath.Join(c.metadataDir, combinedID+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false
	}

	var movie scraper.Movie
	if err := json.Unmarshal(data, &movie); err != nil {
		return nil, false
	}

	return &movie, true
}

// SetMovie saves a Movie metadata record to disk.
func (c *Cache) SetMovie(movie *scraper.Movie) error {
	if c == nil || movie == nil {
		return nil
	}
	combinedID := movie.CombinedID
	if combinedID == "" {
		combinedID = scraper.NormalizeToCombinedID(movie.ID)
	}
	if combinedID == "" {
		return fmt.Errorf("cannot cache movie with empty ID")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.MarshalIndent(movie, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(c.metadataDir, combinedID+".json")
	return os.WriteFile(filePath, data, 0o644)
}

// GetImage retrieves raw image bytes (JPEG/PNG) from disk cache.
func (c *Cache) GetImage(id string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	combinedID := scraper.NormalizeToCombinedID(id)
	if combinedID == "" {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check jpg, png, jpeg
	for _, ext := range []string{".jpg", ".png", ".jpeg"} {
		filePath := filepath.Join(c.imagesDir, combinedID+"_poster"+ext)
		if data, err := os.ReadFile(filePath); err == nil && len(data) > 0 {
			return data, true
		}
	}

	return nil, false
}

// GetImagePath returns the absolute file path to a cached image if it exists on disk.
func (c *Cache) GetImagePath(id string) (string, bool) {
	if c == nil {
		return "", false
	}
	combinedID := scraper.NormalizeToCombinedID(id)
	if combinedID == "" {
		return "", false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, ext := range []string{".jpg", ".png", ".jpeg"} {
		filePath := filepath.Join(c.imagesDir, combinedID+"_poster"+ext)
		if stat, err := os.Stat(filePath); err == nil && stat.Size() > 0 {
			return filePath, true
		}
	}
	return "", false
}

// SetImage saves raw image bytes (JPEG/PNG) to disk cache.
func (c *Cache) SetImage(id string, data []byte) error {
	if c == nil || len(data) == 0 {
		return nil
	}
	combinedID := scraper.NormalizeToCombinedID(id)
	if combinedID == "" {
		return fmt.Errorf("cannot cache image with empty ID")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	ext := ".jpg"
	if len(data) > 8 && (strings.HasPrefix(string(data[:8]), "\x89PNG") || strings.HasPrefix(string(data[:4]), "\x89PNG")) {
		ext = ".png"
	}

	filePath := filepath.Join(c.imagesDir, combinedID+"_poster"+ext)
	return os.WriteFile(filePath, data, 0o644)
}

// Clear removes all cached files.
func (c *Cache) Clear() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	_ = os.RemoveAll(c.metadataDir)
	_ = os.RemoveAll(c.imagesDir)
	_ = os.MkdirAll(c.metadataDir, 0o755)
	_ = os.MkdirAll(c.imagesDir, 0o755)
	return nil
}
