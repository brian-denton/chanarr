// Package config handles startup configuration and environment checks. See
// spec.md §1: ffmpeg/ffprobe are chanarr's one external runtime dependency,
// checked for at startup with a clear error if missing.
package config

import "os/exec"

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

// Config is chanarr's runtime configuration. TODO: populate from flags/env
// (listen address, SQLite path, media root, etc.).
type Config struct {
	ListenAddr string
	DBPath     string
	// LogosDir is where internal/httpapi stores uploaded channel logos.
	LogosDir string
}

// Load builds a Config from flags/environment. TODO: implement.
func Load() Config {
	return Config{
		ListenAddr: ":5004",
		DBPath:     "chanarr.db",
		LogosDir:   "logos",
	}
}
