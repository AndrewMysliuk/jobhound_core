package config

import (
	"os"
	"strconv"
	"strings"
)

// Himalayas collector env (specs/005-job-collectors/contracts/environment.md).
const (
	EnvCollectorHimalayasDisabled  = "JOBHOUND_COLLECTOR_HIMALAYAS_DISABLED"
	EnvCollectorHimalayasMaxPages  = "JOBHOUND_COLLECTOR_HIMALAYAS_MAX_PAGES"
	EnvCollectorHimalayasSearch    = "JOBHOUND_COLLECTOR_HIMALAYAS_SEARCH"
	defaultCollectorHimalayasPages = 0
)

// HimalayasCollectorConfig toggles and caps the Himalayas JSON collector.
type HimalayasCollectorConfig struct {
	Disabled bool
	// MaxPages: 0 → collector default; -1 → unlimited pages; >0 → cap.
	MaxPages int
	// Search is the free-text q for GET …/jobs/api/search (same role as DouCollectorConfig.Search).
	// Empty → browse full JSON feed; non-empty → search mode only.
	Search string
}

// LoadHimalayasCollectorFromEnv reads JOBHOUND_COLLECTOR_HIMALAYAS_* variables.
func LoadHimalayasCollectorFromEnv() HimalayasCollectorConfig {
	cfg := HimalayasCollectorConfig{
		MaxPages: defaultCollectorHimalayasPages,
		Search:   strings.TrimSpace(os.Getenv(EnvCollectorHimalayasSearch)),
	}
	if v := strings.TrimSpace(os.Getenv(EnvCollectorHimalayasDisabled)); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			cfg.Disabled = true
		}
	}
	if v := strings.TrimSpace(os.Getenv(EnvCollectorHimalayasMaxPages)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxPages = n
		}
	}
	return cfg
}
