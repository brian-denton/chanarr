package library

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// requireFFmpeg skips the test when ffmpeg/ffprobe aren't on PATH, so this
// suite stays portable — internal/config.CheckFFmpeg is what guarantees
// their presence for a real chanarr run.
func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not on PATH")
	}
}

func TestFfprobeDuration_RealClip(t *testing.T) {
	requireFFmpeg(t)

	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.ts")
	cmd := exec.Command("ffmpeg",
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x240:rate=25:duration=2",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac",
		"-f", "mpegts", clip,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate test clip: %v\n%s", err, out)
	}

	d, err := ffprobeDuration(clip)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d < 1900*time.Millisecond || d > 2100*time.Millisecond {
		t.Errorf("duration = %v, want ~2s", d)
	}
}

func TestFfprobeDuration_MissingFile(t *testing.T) {
	requireFFmpeg(t)

	_, err := ffprobeDuration(filepath.Join(t.TempDir(), "does-not-exist.mkv"))
	if err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}
