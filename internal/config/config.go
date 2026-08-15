// Package config handles startup configuration and environment checks. See
// spec.md §1: ffmpeg/ffprobe are chanarr's one external runtime dependency,
// checked for at startup with a clear error if missing.
package config

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// CheckFFmpeg verifies ffmpeg and ffprobe are on PATH, returning a clear
// error naming whichever is missing.
func CheckFFmpeg() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return errMissingFFmpeg("ffmpeg")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return errMissingFFmpeg("ffprobe")
	}
	return nil
}

type errMissingFFmpeg string

func (e errMissingFFmpeg) Error() string {
	return string(e) + " not found on PATH — chanarr requires ffmpeg and ffprobe to be installed"
}

// Config is chanarr's runtime configuration.
type Config struct {
	ListenAddr string
	// DataDir is the stable per-user directory everything persistent lives
	// under — fixed regardless of the launch directory, so restarting the
	// server (from anywhere) always finds the same database and logos.
	DataDir string
	DBPath  string
	// LogosDir is where internal/httpapi stores uploaded channel logos.
	LogosDir string
}

// Environment overrides. CHANARR_DATA_DIR relocates all persistent state
// (useful for tests, Docker volumes, or running several instances);
// CHANARR_ADDR overrides the listen address.
const (
	envDataDir = "CHANARR_DATA_DIR"
	envAddr    = "CHANARR_ADDR"
)

// Load resolves the runtime configuration. The data directory defaults to
// the platform's per-user config location (~/Library/Application Support/
// chanarr on macOS, ~/.config/chanarr on Linux) — never the launch
// directory, which earlier versions used and which silently "lost" all
// settings whenever the server was started from somewhere else. A legacy
// ./chanarr.db (plus ./logos) from those versions is migrated in on first
// run.
func Load() (Config, error) {
	addr := os.Getenv(envAddr)
	if addr == "" {
		addr = ":5004"
	}

	dataDir := os.Getenv(envDataDir)
	if dataDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return Config{}, fmt.Errorf("config: no %s set and no user config dir: %w", envDataDir, err)
		}
		dataDir = filepath.Join(base, "chanarr")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("config: create data dir: %w", err)
	}

	cfg := Config{
		ListenAddr: addr,
		DataDir:    dataDir,
		DBPath:     filepath.Join(dataDir, "chanarr.db"),
		LogosDir:   filepath.Join(dataDir, "logos"),
	}
	if err := migrateLegacy(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// migrateLegacy copies a launch-directory chanarr.db (and logos/) from the
// pre-data-dir era into the data dir, once — only when the data dir holds
// no database yet. The legacy files are left in place (copy, not move):
// worthless clutter is safer than a destructive migration step.
func migrateLegacy(cfg Config) error {
	if _, err := os.Stat(cfg.DBPath); err == nil {
		return nil // the data dir is already in use
	}
	legacyDB := "chanarr.db"
	if _, err := os.Stat(legacyDB); err != nil {
		return nil // nothing to migrate
	}

	if err := copyFile(legacyDB, cfg.DBPath); err != nil {
		return fmt.Errorf("config: migrate legacy database: %w", err)
	}
	fmt.Fprintf(os.Stderr, "chanarr: migrated ./%s into %s\n", legacyDB, cfg.DataDir)

	entries, err := os.ReadDir("logos")
	if err != nil {
		return nil // no legacy logos — done
	}
	if err := os.MkdirAll(cfg.LogosDir, 0o755); err != nil {
		return fmt.Errorf("config: migrate legacy logos: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join("logos", e.Name())
		if err := copyFile(src, filepath.Join(cfg.LogosDir, e.Name())); err != nil {
			return fmt.Errorf("config: migrate legacy logo %s: %w", src, err)
		}
	}
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dest)
		return err
	}
	return out.Close()
}
