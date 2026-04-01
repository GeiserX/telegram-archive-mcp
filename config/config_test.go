package config

import (
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("TELEGRAM_ARCHIVE_URL", "")
	t.Setenv("TELEGRAM_ARCHIVE_USER", "")
	t.Setenv("TELEGRAM_ARCHIVE_PASS", "")
	cfg := Load()
	if cfg.BaseURL != "http://localhost:3000" {
		t.Errorf("BaseURL default: got %q, want %q", cfg.BaseURL, "http://localhost:3000")
	}
	if cfg.User != "" {
		t.Errorf("User default: got %q, want empty", cfg.User)
	}
	if cfg.Pass != "" {
		t.Errorf("Pass default: got %q, want empty", cfg.Pass)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("TELEGRAM_ARCHIVE_URL", "http://archive:5000")
	t.Setenv("TELEGRAM_ARCHIVE_USER", "admin")
	t.Setenv("TELEGRAM_ARCHIVE_PASS", "secret123")
	cfg := Load()
	if cfg.BaseURL != "http://archive:5000" {
		t.Errorf("BaseURL: got %q, want %q", cfg.BaseURL, "http://archive:5000")
	}
	if cfg.User != "admin" {
		t.Errorf("User: got %q, want %q", cfg.User, "admin")
	}
	if cfg.Pass != "secret123" {
		t.Errorf("Pass: got %q, want %q", cfg.Pass, "secret123")
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envVal   string
		fallback string
		want     string
	}{
		{
			name:     "returns env value when set",
			key:      "TEST_TGARCHIVE_VAR",
			envVal:   "custom",
			fallback: "default",
			want:     "custom",
		},
		{
			name:     "returns default when env empty",
			key:      "TEST_TGARCHIVE_EMPTY",
			envVal:   "",
			fallback: "fallback",
			want:     "fallback",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.key, tc.envVal)
			got := getEnv(tc.key, tc.fallback)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
