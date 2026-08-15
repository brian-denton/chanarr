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

func generateTestClip(t *testing.T, path string) {
	t.Helper()
	cmd := exec.Command("ffmpeg",
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x240:rate=25:duration=2",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-ar", "48000", "-ac", "2",
		"-f", "mpegts", path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate test clip: %v\n%s", err, out)
	}
}

func TestFfprobeFile_RealClip(t *testing.T) {
	requireFFmpeg(t)

	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.ts")
	generateTestClip(t, clip)

	d, params, err := ffprobeFile(clip)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d < 1900*time.Millisecond || d > 2100*time.Millisecond {
		t.Errorf("duration = %v, want ~2s", d)
	}
	if params.VideoCodec != "h264" {
		t.Errorf("VideoCodec = %q, want h264", params.VideoCodec)
	}
	if params.Width != 320 || params.Height != 240 {
		t.Errorf("dimensions = %dx%d, want 320x240", params.Width, params.Height)
	}
	if params.FrameRate < 24.9 || params.FrameRate > 25.1 {
		t.Errorf("FrameRate = %v, want ~25", params.FrameRate)
	}
	if params.AudioCodec != "aac" {
		t.Errorf("AudioCodec = %q, want aac", params.AudioCodec)
	}
	if params.AudioChannels != 2 {
		t.Errorf("AudioChannels = %d, want 2", params.AudioChannels)
	}
	if params.AudioSampleRate != 48000 {
		t.Errorf("AudioSampleRate = %d, want 48000", params.AudioSampleRate)
	}
}

func TestFfprobeFile_MissingFile(t *testing.T) {
	requireFFmpeg(t)

	_, _, err := ffprobeFile(filepath.Join(t.TempDir(), "does-not-exist.mkv"))
	if err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}

func TestParseFrameRate(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"25/1", 25},
		{"24000/1001", 23.976023976023978},
		{"30000/1001", 29.97002997002997},
		{"", 0},
		{"garbage", 0},
		{"25/0", 0},
	}
	for _, c := range cases {
		got := parseFrameRate(c.in)
		if diff := got - c.want; diff > 0.0001 || diff < -0.0001 {
			t.Errorf("parseFrameRate(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
