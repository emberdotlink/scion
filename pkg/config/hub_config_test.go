package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultGlobalConfig(t *testing.T) {
	cfg := DefaultGlobalConfig()

	if cfg.Hub.Port != 9810 {
		t.Errorf("expected Hub port 9810, got %d", cfg.Hub.Port)
	}

	if cfg.Hub.Host != "0.0.0.0" {
		t.Errorf("expected Hub host '0.0.0.0', got %q", cfg.Hub.Host)
	}

	if cfg.Hub.ReadTimeout != 30*time.Second {
		t.Errorf("expected ReadTimeout 30s, got %v", cfg.Hub.ReadTimeout)
	}

	if cfg.Hub.WriteTimeout != 60*time.Second {
		t.Errorf("expected WriteTimeout 60s, got %v", cfg.Hub.WriteTimeout)
	}

	if !cfg.Hub.CORSEnabled {
		t.Error("expected CORS to be enabled by default")
	}

	if cfg.Database.Driver != "sqlite" {
		t.Errorf("expected database driver 'sqlite', got %q", cfg.Database.Driver)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("expected log level 'info', got %q", cfg.LogLevel)
	}
}

func TestLoadGlobalConfigDefaults(t *testing.T) {
	// Load config without any config file
	cfg, err := LoadGlobalConfig("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Should have default values
	if cfg.Hub.Port != 9810 {
		t.Errorf("expected Hub port 9810, got %d", cfg.Hub.Port)
	}

	if cfg.Database.Driver != "sqlite" {
		t.Errorf("expected database driver 'sqlite', got %q", cfg.Database.Driver)
	}

	// Database URL should be set to default path
	if cfg.Database.URL == "" {
		t.Error("expected database URL to be set")
	}
}

func TestLoadGlobalConfigFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "server.yaml")

	configContent := `
hub:
  port: 8080
  host: "127.0.0.1"
  corsEnabled: false

database:
  driver: postgres
  url: "postgres://localhost:5432/scion"

logLevel: debug
logFormat: json
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadGlobalConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Hub.Port != 8080 {
		t.Errorf("expected Hub port 8080, got %d", cfg.Hub.Port)
	}

	if cfg.Hub.Host != "127.0.0.1" {
		t.Errorf("expected Hub host '127.0.0.1', got %q", cfg.Hub.Host)
	}

	if cfg.Hub.CORSEnabled {
		t.Error("expected CORS to be disabled")
	}

	if cfg.Database.Driver != "postgres" {
		t.Errorf("expected database driver 'postgres', got %q", cfg.Database.Driver)
	}

	if cfg.Database.URL != "postgres://localhost:5432/scion" {
		t.Errorf("expected database URL 'postgres://localhost:5432/scion', got %q", cfg.Database.URL)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level 'debug', got %q", cfg.LogLevel)
	}

	if cfg.LogFormat != "json" {
		t.Errorf("expected log format 'json', got %q", cfg.LogFormat)
	}
}

func TestLoadGlobalConfigFromDirectory(t *testing.T) {
	// Create a temporary directory with config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "server.yaml")

	configContent := `
hub:
  port: 9999
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Load from directory (not file path)
	cfg, err := LoadGlobalConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Hub.Port != 9999 {
		t.Errorf("expected Hub port 9999, got %d", cfg.Hub.Port)
	}
}

func TestLoadGlobalConfigEnvOverride(t *testing.T) {
	// Set environment variables
	// Note: Env vars use underscores which map to dots for nesting
	os.Setenv("SCION_SERVER_HUB_PORT", "7777")
	os.Setenv("SCION_SERVER_DATABASE_DRIVER", "postgres")
	defer func() {
		os.Unsetenv("SCION_SERVER_HUB_PORT")
		os.Unsetenv("SCION_SERVER_DATABASE_DRIVER")
	}()

	cfg, err := LoadGlobalConfig("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Hub.Port != 7777 {
		t.Errorf("expected Hub port 7777 from env, got %d", cfg.Hub.Port)
	}

	if cfg.Database.Driver != "postgres" {
		t.Errorf("expected database driver 'postgres' from env, got %q", cfg.Database.Driver)
	}
}
