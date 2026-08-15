package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ResolvesStableDataDirAndOverrides(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "chanarr-data")
	t.Setenv(envDataDir, dataDir)
	t.Setenv(envAddr, ":9999")
	t.Chdir(t.TempDir()) // launch directory must not matter

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ListenAddr != ":9999" {
		t.Errorf("ListenAddr = %q, want env override", cfg.ListenAddr)
	}
	if cfg.DBPath != filepath.Join(dataDir, "chanarr.db") {
		t.Errorf("DBPath = %q, want inside the data dir", cfg.DBPath)
	}
	if cfg.LogosDir != filepath.Join(dataDir, "logos") {
		t.Errorf("LogosDir = %q, want inside the data dir", cfg.LogosDir)
	}
	if _, err := os.Stat(dataDir); err != nil {
		t.Errorf("data dir was not created: %v", err)
	}
}

func TestLoad_MigratesLegacyLaunchDirDatabaseOnce(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	t.Setenv(envDataDir, dataDir)
	launchDir := t.TempDir()
	t.Chdir(launchDir)

	// A pre-data-dir setup: db + one uploaded logo, relative to the cwd.
	if err := os.WriteFile("chanarr.db", []byte("legacy-db-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("logos", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("logos", "1.png"), []byte("logo"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(cfg.DBPath)
	if err != nil || string(got) != "legacy-db-bytes" {
		t.Fatalf("migrated db = %q, %v", got, err)
	}
	logo, err := os.ReadFile(filepath.Join(cfg.LogosDir, "1.png"))
	if err != nil || string(logo) != "logo" {
		t.Fatalf("migrated logo = %q, %v", logo, err)
	}
	// The legacy copy stays put — migration must never destroy anything.
	if _, err := os.Stat("chanarr.db"); err != nil {
		t.Error("legacy db must be left in place")
	}

	// Second run with different legacy content: the data dir now holds
	// state, so nothing may be overwritten.
	if err := os.WriteFile("chanarr.db", []byte("newer-legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ = os.ReadFile(cfg.DBPath)
	if string(got) != "legacy-db-bytes" {
		t.Errorf("second Load overwrote the data-dir db with %q", got)
	}
}
