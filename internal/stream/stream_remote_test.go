package stream

import (
	"crypto/md5"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	gonfs "github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"

	"chanarr/internal/netfs"
	"chanarr/internal/schedule"
)

// TestHandler_StreamsRemoteItemsViaBridge is the end-to-end proof that
// items living on a network share actually play: two real clips served by
// a real (in-process, pure-Go) NFS server, streamed by real ffmpeg that
// reads them through the netfs loopback bridge. It exercises both phases
// against http inputs — phase 1's mid-item -ss seek over a bridge URL
// (Range-backed), and phase 2's concat playlist whose entries are bridge
// URLs (the -protocol_whitelist path). Verified the same way as the local
// wraparound test: decoded frames classified against reference hashes.
func TestHandler_StreamsRemoteItemsViaBridge(t *testing.T) {
	requireFFmpeg(t)

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Media", "Show"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Distinct patterns; classification is by membership in each clip's
	// full decoded-frame hash set (frames vary slightly even for a static
	// pattern — rate control gives each frame its own quantization).
	paramsA := generateClip(t, filepath.Join(dir, "Media", "Show", "a1.mkv"), "smptehdbars", 3, 600, 640, 360, 25)
	generateClip(t, filepath.Join(dir, "Media", "Show", "a2.mkv"), "rgbtestsrc", 3, 1200, 640, 360, 25)
	refA := hashSet(rawFrameHashes(t, filepath.Join(dir, "Media", "Show", "a1.mkv"), 640, 360))
	refB := hashSet(rawFrameHashes(t, filepath.Join(dir, "Media", "Show", "a2.mkv"), 640, 360))
	for h := range refA {
		if refB[h] {
			t.Fatal("reference clips share decoded frames — not distinguishable")
		}
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go gonfs.Serve(ln, nfshelper.NewCachingHandler(nfshelper.NewNullAuthHandler(osfs.New(dir)), 1024)) //nolint:errcheck
	host := ln.Addr().String()

	mgr := netfs.NewManager(nil)
	if err := mgr.Start(); err != nil {
		t.Fatal(err)
	}

	epoch := schedule.Epoch{
		Start: time.Now().Add(-1500 * time.Millisecond), // mid-item 1: forces a phase-1 seek
		Items: []schedule.PlaylistItem{
			{Path: "nfs://" + host + "/Media/Show/a1.mkv", Duration: 3 * time.Second, Params: paramsA},
			{Path: "nfs://" + host + "/Media/Show/a2.mkv", Duration: 3 * time.Second, Params: paramsA},
		},
	}
	provider := func(number string) (schedule.Channel, schedule.Epoch, error) {
		return schedule.Channel{Number: number}, epoch, nil
	}

	capturePath := captureStream(t, provider, mgr.InputTarget, "1", 7*time.Second)

	// Classify every decoded frame, sequentially (rawFrameHashes) rather
	// than by seek-extraction: phase 1 preserves the source's original TS
	// timestamps from the seek point while phase 2 restarts at its
	// analytic offset, so PTS overlap at the boundary and seeking within
	// the capture is unreliable — but sequential decode ignores PTS. The
	// capture must open on clip A (phase 1, -ss over a bridge URL) and
	// reach clip B (phase 2, concat of bridge URLs) — anything else means
	// a phase failed and filler or nothing was served.
	frames := rawFrameHashes(t, capturePath, 640, 360)
	var sequence []byte
	for _, h := range frames {
		switch {
		case refA[h]:
			sequence = append(sequence, 'A')
		case refB[h]:
			sequence = append(sequence, 'B')
		default:
			sequence = append(sequence, '?')
		}
	}
	got := compressRuns(sequence)
	if len(got) == 0 || got[0] != 'A' {
		t.Fatalf("capture must open on clip A's remainder (phase 1 over the bridge); run-compressed sequence: %q", got)
	}
	if !strings.Contains(got, "B") {
		t.Fatalf("capture never reached clip B (phase 2 concat over the bridge failed); run-compressed sequence: %q", got)
	}
	if strings.Contains(got, "?") {
		t.Fatalf("capture contains frames matching neither clip (filler = a phase failed); run-compressed sequence: %q", got)
	}
}

// rawFrameHashes decodes the whole capture front to back into raw yuv420p
// frames and returns each frame's MD5.
func rawFrameHashes(t *testing.T, path string, w, h int) []string {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-v", "error", "-i", path, "-f", "rawvideo", "-pix_fmt", "yuv420p", "-").Output()
	if err != nil {
		t.Fatalf("decode capture: %v", err)
	}
	frameSize := w * h * 3 / 2
	var hashes []string
	for i := 0; i+frameSize <= len(out); i += frameSize {
		sum := md5.Sum(out[i : i+frameSize])
		hashes = append(hashes, string(sum[:]))
	}
	if len(hashes) == 0 {
		t.Fatal("capture decoded to zero frames")
	}
	return hashes
}

func hashSet(hashes []string) map[string]bool {
	set := make(map[string]bool, len(hashes))
	for _, h := range hashes {
		set[h] = true
	}
	return set
}

// compressRuns collapses repeated classifications ("AAAABBB?" -> "AB?") so
// failure messages stay readable.
func compressRuns(seq []byte) string {
	var out []byte
	for i, c := range seq {
		if i == 0 || seq[i-1] != c {
			out = append(out, c)
		}
	}
	return string(out)
}
