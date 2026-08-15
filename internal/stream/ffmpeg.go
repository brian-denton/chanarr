package stream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"chanarr/internal/schedule"
)

// codecArgs returns the -c/-vf arguments for either remux (-c copy) or
// transcode (decode + re-encode, normalized to target's resolution/fps/
// audio format via an explicit scale+pad+fps filter chain). Just switching
// codec mode without that filter chain would break the moment two items
// actually differ in size or frame rate, since a single encoded output
// stream can't change dimensions mid-stream.
func codecArgs(remux bool, target schedule.StreamParams) []string {
	if remux {
		return []string{"-c", "copy"}
	}
	vf := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,fps=%g",
		target.Width, target.Height, target.Width, target.Height, target.FrameRate,
	)
	return []string{
		"-vf", vf, "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
		"-ar", strconv.Itoa(target.AudioSampleRate), "-ac", strconv.Itoa(target.AudioChannels), "-c:a", "aac",
	}
}

// singleItemCmd plays the remainder of one file from seek onward — phase 1
// of a streaming cycle, joining mid-item at tune-in. A plain single-file
// -ss seek is used deliberately instead of the concat demuxer's own
// inpoint/seek support: empirically, on this ffmpeg build, inpoint (and
// even a whole-input -ss over a concat source) barely trims anything under
// -c copy, while a single-file -ss reliably does. -re throttles input
// reads to realtime — without it a cheap remux would blast out far faster
// than playback speed.
func singleItemCmd(ctx context.Context, path string, seek time.Duration, remux bool, target schedule.StreamParams) *exec.Cmd {
	args := []string{"-hide_banner", "-loglevel", "error", "-re"}
	if seek > 0 {
		args = append(args, "-ss", formatSeconds(seek))
	}
	args = append(args, "-i", path)
	args = append(args, codecArgs(remux, target)...)
	args = append(args, "-f", "mpegts", "pipe:1")
	return exec.CommandContext(ctx, "ffmpeg", args...)
}

// remainderCmd plays a concat playlist of whatever comes after the
// currently-airing item (see buildRemainderPlaylist — never wraps around).
// No seek is needed since it always starts at a clean file boundary.
// tsOffset shifts its output timestamps to continue where singleItemCmd's
// phase left off — computed analytically from the current item's cached
// Duration minus seek, not by probing actual output, so no ffprobe
// round-trip is needed between phases.
func remainderCmd(ctx context.Context, playlistPath string, tsOffset time.Duration, remux bool, target schedule.StreamParams) *exec.Cmd {
	args := []string{
		"-hide_banner", "-loglevel", "error", "-re",
		// Playlist entries may be loopback http URLs (netfs bridge, for
		// items on SMB/NFS shares); a concat input's nested opens are
		// restricted to file-ish protocols by default, so http/tcp must
		// be whitelisted explicitly. Harmless for all-local playlists.
		"-protocol_whitelist", "file,http,https,tcp,tls,crypto,data",
		"-f", "concat", "-safe", "0", "-i", playlistPath,
	}
	args = append(args, codecArgs(remux, target)...)
	args = append(args, "-output_ts_offset", formatSeconds(tsOffset), "-f", "mpegts", "pipe:1")
	return exec.CommandContext(ctx, "ffmpeg", args...)
}

// fillerCmd generates a synthetic black/silent MPEG-TS stream of exactly
// duration — used when an item fails to play, so the TS connection never
// just dies (spec.md §7) while still consuming exactly that item's
// declared duration, keeping every future Airing boundary stable (spec.md
// §2). No burned-in "unavailable" label: this ffmpeg build has no drawtext
// filter available (confirmed against the walking-skeleton prototype); a
// plain black/silent filler is an honest, low-risk v1 default.
func fillerCmd(ctx context.Context, duration time.Duration) *exec.Cmd {
	seconds := duration.Seconds()
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-re",
		"-f", "lavfi", "-i", fmt.Sprintf("color=c=black:s=1280x720:r=25:d=%.3f", seconds),
		"-f", "lavfi", "-i", fmt.Sprintf("anullsrc=r=48000:cl=stereo:d=%.3f", seconds),
		"-t", fmt.Sprintf("%.3f", seconds),
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-f", "mpegts", "pipe:1",
	}
	return exec.CommandContext(ctx, "ffmpeg", args...)
}

func formatSeconds(d time.Duration) string {
	return fmt.Sprintf("%.3f", d.Seconds())
}

// runToWriter pipes cmd's stdout to w and runs it to completion. If ffmpeg
// exits non-zero, its captured stderr (otherwise silently discarded to
// /dev/null by os/exec when Stderr is left nil) is folded into the
// returned error so the caller's log line actually says what went wrong.
func runToWriter(cmd *exec.Cmd, w io.Writer) error {
	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
