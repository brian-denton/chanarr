package library

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// probeDuration is a package-level var (not a plain function) so tests can
// substitute a fake without depending on ffprobe actually being installed.
var probeDuration = ffprobeDuration

// ffprobeDuration shells out to ffprobe to read a file's duration once, at
// scan time — internal/config.CheckFFmpeg has already verified ffprobe is
// on PATH before chanarr starts.
func ffprobeDuration(path string) (time.Duration, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}

	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("ffprobe: parse duration %q: %w", out, err)
	}
	if seconds <= 0 {
		return 0, fmt.Errorf("ffprobe: non-positive duration %v", seconds)
	}
	return time.Duration(seconds * float64(time.Second)), nil
}
