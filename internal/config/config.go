package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config holds all runtime configuration for the patrol dispatch service.
type Config struct {
	ListenAddr              string        `json:"listen_addr"`
	EscalationTimeout       time.Duration `json:"escalation_timeout"`
	EscalationCheckInterval time.Duration `json:"escalation_check_interval"`
	SyncReplayInterval      time.Duration `json:"sync_replay_interval"`
	SyncMaxAttempts         int           `json:"sync_max_attempts"`
	EscalationBureau        string        `json:"escalation_bureau"`
}

// Default returns production-appropriate defaults matching the business rules.
func Default() Config {
	return Config{
		ListenAddr:              ":53649",
		EscalationTimeout:       10 * time.Minute,
		EscalationCheckInterval: 30 * time.Second,
		SyncReplayInterval:      15 * time.Second,
		SyncMaxAttempts:         5,
		EscalationBureau:        "保护区管理局",
	}
}

// rawConfig is an intermediate representation used during JSON loading so
// that duration fields expressed as strings (e.g. "10m") can be parsed
// with time.ParseDuration before assignment to the typed Config.
type rawConfig struct {
	ListenAddr              string `json:"listen_addr"`
	EscalationTimeout       string `json:"escalation_timeout"`
	EscalationCheckInterval string `json:"escalation_check_interval"`
	SyncReplayInterval      string `json:"sync_replay_interval"`
	SyncMaxAttempts         int    `json:"sync_max_attempts"`
	EscalationBureau        string `json:"escalation_bureau"`
}

func parseDuration(field, raw string, fallback time.Duration) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", field, err)
	}
	return d, nil
}

// Load reads configuration from a JSON file path, falling back to defaults
// for any field that is missing or empty.
func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}

	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}

	if raw.ListenAddr != "" {
		cfg.ListenAddr = raw.ListenAddr
	}
	if raw.EscalationBureau != "" {
		cfg.EscalationBureau = raw.EscalationBureau
	}
	if raw.SyncMaxAttempts > 0 {
		cfg.SyncMaxAttempts = raw.SyncMaxAttempts
	}

	if cfg.EscalationTimeout, err = parseDuration("escalation_timeout", raw.EscalationTimeout, cfg.EscalationTimeout); err != nil {
		return cfg, err
	}
	if cfg.EscalationCheckInterval, err = parseDuration("escalation_check_interval", raw.EscalationCheckInterval, cfg.EscalationCheckInterval); err != nil {
		return cfg, err
	}
	if cfg.SyncReplayInterval, err = parseDuration("sync_replay_interval", raw.SyncReplayInterval, cfg.SyncReplayInterval); err != nil {
		return cfg, err
	}

	return cfg, nil
}
