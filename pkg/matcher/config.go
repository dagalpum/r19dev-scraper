package matcher

// Config holds matcher configuration options.
type Config struct {
	CustomRegexEnabled bool
	CustomRegexPattern string
	StripNoisePrefixes bool
}

// DefaultConfig returns the default matcher configuration.
func DefaultConfig() *Config {
	return &Config{
		CustomRegexEnabled: false,
		CustomRegexPattern: "",
		StripNoisePrefixes: true,
	}
}
