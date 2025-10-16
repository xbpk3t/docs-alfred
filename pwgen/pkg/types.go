package pwgen

// Config represents password generation configuration.
type Config struct {
	SecretKey     string
	Length        int
	IsUppercase   bool
	IsNum         bool
	IsPunctuation bool
}

// NewConfig creates a new password generation configuration.
func NewConfig(secretKey string, length int, uppercase, numbers, punctuation bool) *Config {
	return &Config{
		SecretKey:     secretKey,
		Length:        length,
		IsUppercase:   uppercase,
		IsNum:         numbers,
		IsPunctuation: punctuation,
	}
}
