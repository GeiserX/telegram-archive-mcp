package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func init() {
	if strings.ToLower(os.Getenv("TRANSPORT")) != "stdio" {
		_ = godotenv.Load()
	}
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
