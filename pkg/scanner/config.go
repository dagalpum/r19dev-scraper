package scanner

// Config holds scanner configuration options.
type Config struct {
	Extensions      []string // Allowed video extensions, e.g. [".mp4", ".mkv"]
	MinSizeMB       int      // Minimum file size in MB (0 = no minimum)
	ExcludePatterns []string // Glob patterns to ignore, e.g. ["*sample*", "*trailer*"]
	MaxFiles        int      // Maximum files to discover (0 = unlimited)
}

// DefaultConfig returns reasonable default scanner configuration.
func DefaultConfig() *Config {
	return &Config{
		Extensions:      []string{".mp4", ".mkv", ".avi", ".wmv", ".flv", ".iso", ".ts", ".m4v", ".mov"},
		MinSizeMB:       50,
		ExcludePatterns: []string{"*sample*", "*trailer*", "*preview*", "*.url", "*.txt"},
		MaxFiles:        0,
	}
}
