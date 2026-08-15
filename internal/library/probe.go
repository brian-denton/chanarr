package library

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"chanarr/internal/schedule"
)

// probeFile is a package-level var (not a plain function) so tests can
// substitute a fake without depending on ffprobe actually being installed.
var probeFile = ffprobeFile

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecName  string `json:"codec_name"`
	CodecType  string `json:"codec_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	RFrameRate string `json:"r_frame_rate"`
	Channels   int    `json:"channels"`
	SampleRate string `json:"sample_rate"`
}

type ffprobeFormat struct {
	Duration string `json:"duration"`
}

// ffprobeFile shells out to ffprobe once per file, at scan time, to read
// both its duration and the stream parameters internal/stream's
// remux-vs-transcode gate needs — internal/config.CheckFFmpeg has already
// verified ffprobe is on PATH before chanarr starts.
func ffprobeFile(path string) (time.Duration, schedule.StreamParams, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_entries", "format=duration:stream=codec_name,codec_type,width,height,r_frame_rate,channels,sample_rate",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, schedule.StreamParams{}, fmt.Errorf("ffprobe: %w", err)
	}

	var parsed ffprobeOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return 0, schedule.StreamParams{}, fmt.Errorf("ffprobe: parse output: %w", err)
	}

	seconds, err := strconv.ParseFloat(parsed.Format.Duration, 64)
	if err != nil {
		return 0, schedule.StreamParams{}, fmt.Errorf("ffprobe: parse duration %q: %w", parsed.Format.Duration, err)
	}
	if seconds <= 0 {
		return 0, schedule.StreamParams{}, fmt.Errorf("ffprobe: non-positive duration %v", seconds)
	}
	duration := time.Duration(seconds * float64(time.Second))

	var params schedule.StreamParams
	for _, s := range parsed.Streams {
		switch s.CodecType {
		case "video":
			if params.VideoCodec == "" { // first video stream only
				params.VideoCodec = s.CodecName
				params.Width = s.Width
				params.Height = s.Height
				params.FrameRate = parseFrameRate(s.RFrameRate)
			}
		case "audio":
			if params.AudioCodec == "" { // first audio stream only
				params.AudioCodec = s.CodecName
				params.AudioChannels = s.Channels
				sampleRate, _ := strconv.Atoi(s.SampleRate)
				params.AudioSampleRate = sampleRate
			}
		}
	}

	return duration, params, nil
}

// parseFrameRate converts ffprobe's "num/den" r_frame_rate (e.g. "25/1",
// "24000/1001") into frames per second. Malformed or zero-denominator
// input yields 0, which simply never matches another file's frame rate —
// safe under the remux-compatibility gate (spec.md §7: any mismatch just
// forces that comparison to transcode, never a hard failure).
func parseFrameRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		return 0
	}
	n, err1 := strconv.ParseFloat(num, 64)
	d, err2 := strconv.ParseFloat(den, 64)
	if err1 != nil || err2 != nil || d == 0 {
		return 0
	}
	return n / d
}
