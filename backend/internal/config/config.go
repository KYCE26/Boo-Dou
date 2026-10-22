package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port       string
	LLMBaseURL string
	LLMAPIKey  string
	LLMModel   string
	// Optional, provider-specific note such as "tengah malam WIB".
	LLMQuotaReset string
	// LLMVision says whether the model accepts images; the UI hides uploads when false.
	LLMVision   bool
	CORSOrigin  string
	DatabaseURL string
	// UploadDir is relative to the backend working directory unless absolute.
	UploadDir string

	visionRaw string
}

// Load reads .env if present; real environment variables win so deployments
// can override the file without editing it.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("gagal membaca .env: %w", err)
	}

	cfg := &Config{
		Port:          envOr("PORT", "8080"),
		LLMBaseURL:    strings.TrimRight(strings.TrimSpace(os.Getenv("LLM_BASE_URL")), "/"),
		LLMAPIKey:     strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		LLMModel:      strings.TrimSpace(os.Getenv("LLM_MODEL")),
		LLMQuotaReset: strings.TrimSpace(os.Getenv("LLM_QUOTA_RESET")),
		CORSOrigin:    strings.TrimRight(strings.TrimSpace(os.Getenv("CORS_ORIGIN")), "/"),
		DatabaseURL:   strings.TrimSpace(os.Getenv("DATABASE_URL")),
		UploadDir:     envOr("UPLOAD_DIR", "data/uploads"),
		visionRaw:     strings.TrimSpace(os.Getenv("LLM_VISION")),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LLM_API_KEY is deliberately not required here: the server should still boot
// for health checks and UI work, and /api/chat reports the missing key itself.
func (c *Config) validate() error {
	var problems []string

	if p, err := strconv.Atoi(c.Port); err != nil || p < 1 || p > 65535 {
		problems = append(problems, "PORT harus angka 1-65535")
	}
	if c.LLMBaseURL == "" {
		problems = append(problems, "LLM_BASE_URL wajib diisi")
	} else if u, err := url.Parse(c.LLMBaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		problems = append(problems, "LLM_BASE_URL harus URL http(s) yang valid")
	}
	if c.LLMModel == "" {
		problems = append(problems, "LLM_MODEL wajib diisi")
	}
	if c.CORSOrigin == "" {
		problems = append(problems, "CORS_ORIGIN wajib diisi")
	}
	if c.DatabaseURL == "" {
		problems = append(problems, "DATABASE_URL wajib diisi")
	}
	// Unset means false: sending images to a text-only model fails at the
	// provider, so vision must be switched on deliberately.
	if c.visionRaw != "" {
		v, err := strconv.ParseBool(c.visionRaw)
		if err != nil {
			problems = append(problems, "LLM_VISION harus true atau false")
		}
		c.LLMVision = v
	}

	if len(problems) > 0 {
		return errors.New("konfigurasi tidak valid: " + strings.Join(problems, "; "))
	}
	return nil
}

func (c *Config) HasAPIKey() bool {
	return c.LLMAPIKey != ""
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
