package scanner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileInfo contains metadata for a discovered video file.
type FileInfo struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Extension string    `json:"extension"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
}

// SizeMB returns file size in megabytes.
func (f FileInfo) SizeMB() float64 {
	return float64(f.Size) / (1024 * 1024)
}

// ScanResult contains all matched files and scan diagnostics.
type ScanResult struct {
	Files        []FileInfo `json:"files"`
	SkippedCount int        `json:"skipped_count"`
	TotalScanned int        `json:"total_scanned"`
	Errors       []error    `json:"errors"`
	TimedOut     bool       `json:"timed_out"`
	LimitReached bool       `json:"limit_reached"`
}

// Scanner performs directory walking and file filtering.
type Scanner struct {
	config *Config
	extSet map[string]struct{}
}

// New creates a new Scanner instance.
func New(cfg *Config) *Scanner {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	extSet := make(map[string]struct{}, len(cfg.Extensions))
	for _, ext := range cfg.Extensions {
		clean := strings.ToLower(strings.TrimSpace(ext))
		if !strings.HasPrefix(clean, ".") {
			clean = "." + clean
		}
		extSet[clean] = struct{}{}
	}
	return &Scanner{
		config: cfg,
		extSet: extSet,
	}
}

// Scan traverses rootPath recursively for valid video files.
func (s *Scanner) Scan(rootPath string) (*ScanResult, error) {
	return s.ScanContext(context.Background(), rootPath)
}

// ScanContext traverses rootPath with context cancellation and timeout support.
func (s *Scanner) ScanContext(ctx context.Context, rootPath string) (*ScanResult, error) {
	return s.ScanStream(ctx, rootPath, 0, nil)
}

// ScanStream walks rootPath and streams discovered files in batches of chunkSize (e.g. 5-10 files) to the provided channel.
func (s *Scanner) ScanStream(ctx context.Context, rootPath string, chunkSize int, out chan<- []FileInfo) (*ScanResult, error) {
	if chunkSize <= 0 {
		chunkSize = 10
	}
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	result := &ScanResult{
		Files:  make([]FileInfo, 0),
		Errors: make([]error, 0),
	}

	// Single file scan
	if !stat.IsDir() {
		lstat, err := os.Lstat(absPath)
		if err != nil {
			return nil, err
		}
		if lstat.Mode()&os.ModeSymlink != 0 {
			result.SkippedCount++
			return result, nil
		}
		if s.shouldIncludeFile(absPath, lstat.Size()) {
			file := FileInfo{
				Path:      absPath,
				Name:      lstat.Name(),
				Extension: filepath.Ext(absPath),
				Size:      lstat.Size(),
				ModTime:   lstat.ModTime(),
			}
			result.Files = append(result.Files, file)
			if out != nil {
				out <- []FileInfo{file}
			}
		} else {
			result.SkippedCount++
		}
		result.TotalScanned = 1
		return result, nil
	}

	var buffer []FileInfo
	fileCount := 0

	err = filepath.WalkDir(absPath, func(path string, d fs.DirEntry, walkErr error) error {
		fileCount++
		if fileCount%50 == 0 {
			select {
			case <-ctx.Done():
				result.TimedOut = true
				return filepath.SkipAll
			default:
			}
		}

		if walkErr != nil {
			result.Errors = append(result.Errors, walkErr)
			return nil
		}

		// Security: Skip symlinks to avoid directory escapes or recursion
		lstat, statErr := os.Lstat(path)
		if statErr != nil {
			result.Errors = append(result.Errors, statErr)
			return nil
		}
		if lstat.Mode()&os.ModeSymlink != 0 {
			result.SkippedCount++
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		result.TotalScanned++

		if s.shouldIncludeFile(path, lstat.Size()) {
			fi := FileInfo{
				Path:      path,
				Name:      d.Name(),
				Extension: filepath.Ext(path),
				Size:      lstat.Size(),
				ModTime:   lstat.ModTime(),
			}
			result.Files = append(result.Files, fi)
			buffer = append(buffer, fi)

			if len(buffer) >= chunkSize {
				if out != nil {
					sendChunk := make([]FileInfo, len(buffer))
					copy(sendChunk, buffer)
					out <- sendChunk
				}
				buffer = buffer[:0]
			}

			if s.config.MaxFiles > 0 && len(result.Files) >= s.config.MaxFiles {
				result.LimitReached = true
				return filepath.SkipAll
			}
		} else {
			result.SkippedCount++
		}

		return nil
	})

	if len(buffer) > 0 && out != nil {
		sendChunk := make([]FileInfo, len(buffer))
		copy(sendChunk, buffer)
		out <- sendChunk
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Scanner) shouldIncludeFile(path string, size int64) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := s.extSet[ext]; !ok {
		return false
	}

	baseName := filepath.Base(path)
	for _, pattern := range s.config.ExcludePatterns {
		matched, err := filepath.Match(pattern, baseName)
		if err == nil && matched {
			return false
		}
	}

	if s.config.MinSizeMB > 0 {
		minBytes := int64(s.config.MinSizeMB) * 1024 * 1024
		if size < minBytes {
			return false
		}
	}

	return true
}
