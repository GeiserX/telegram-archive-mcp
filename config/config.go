package config

import (
	"os"

	"github.com/joho/godotenv"
)

func init() {
	// Load .env in the working directory; ignore error if the file is absent.
	_ = godotenv.Load()
}

type Config struct {
	BaseURL string
	User    string
	Pass    string
}

func Load() Config {
	return Config{
		BaseURL: getEnv("TELEGRAM_ARCHIVE_URL", "http://localhost:3000"),
		User:    getEnv("TELEGRAM_ARCHIVE_USER", ""),
		Pass:    getEnv("TELEGRAM_ARCHIVE_PASS", ""),
	}
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
