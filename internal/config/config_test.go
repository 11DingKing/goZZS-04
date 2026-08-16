package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.ListenAddr != ":53649" {
		t.Fatalf("expected :53649, got %s", cfg.ListenAddr)
	}
	if cfg.EscalationTimeout != 10*time.Minute {
		t.Fatalf("expected 10m timeout, got %v", cfg.EscalationTimeout)
	}
	if cfg.SyncMaxAttempts != 5 {
		t.Fatalf("expected 5 max attempts, got %d", cfg.SyncMaxAttempts)
	}
	if cfg.EscalationBureau == "" {
		t.Fatal("escalation bureau should not be empty")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{
		"listen_addr": ":53649",
		"escalation_timeout": "10m",
		"escalation_check_interval": "30s",
		"sync_replay_interval": "15s",
		"sync_max_attempts": 5,
		"escalation_bureau": "Test Bureau"
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.EscalationTimeout != 10*time.Minute {
		t.Fatalf("expected 10m, got %v", cfg.EscalationTimeout)
	}
	if cfg.EscalationCheckInterval != 30*time.Second {
		t.Fatalf("expected 30s, got %v", cfg.EscalationCheckInterval)
	}
	if cfg.EscalationBureau != "Test Bureau" {
		t.Fatalf("expected Test Bureau, got %s", cfg.EscalationBureau)
	}
}

func TestLoadFromEmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load with empty path should not error: %v", err)
	}
	if cfg.ListenAddr != ":53649" {
		t.Fatalf("expected default :53649, got %s", cfg.ListenAddr)
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"escalation_timeout": "not-a-duration"}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid duration, got nil")
	}
}
