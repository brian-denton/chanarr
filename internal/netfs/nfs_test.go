package netfs

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/osfs"
	gonfs "github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"
)

// startNFSServer serves dir over NFSv3 on a random loopback port using the
// pure-Go go-nfs server — a real NFS server, in-process, no root needed.
// It ignores the MOUNT dirpath and always returns dir as the root; the
// export resolution's shallowest-first + Lookup-verify handles that
// correctly (mounts "/", then finds the real subtree under it), so specs
// here use real subdirectories of dir (e.g. nfs://host/Media/...).
func startNFSServer(t *testing.T, dir string) (host string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	handler := nfshelper.NewNullAuthHandler(osfs.New(dir))
	go gonfs.Serve(ln, nfshelper.NewCachingHandler(handler, 1024)) //nolint:errcheck
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String()
}

func writeTree(t *testing.T, dir string, files map[string][]byte) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNFS_MountWalkReadSeek(t *testing.T) {
	dir := t.TempDir()
	big := bytes.Repeat([]byte("0123456789abcdef"), 4096) // 64KiB, phase-detectable
	writeTree(t, dir, map[string][]byte{
		"Media/Season 1/e1.mkv": []byte("episode one"),
		"Media/Season 1/e2.mkv": big,
		"Media/poster.jpg":      []byte("logo"),
	})
	host := startNFSServer(t, dir)

	mgr := NewManager(nil)
	m, err := mgr.Mount("nfs://" + host + "/Media")
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	defer m.Close()

	var rels []string
	err = fs.WalkDir(m.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rels = append(rels, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	want := []string{"Season 1/e1.mkv", "Season 1/e2.mkv", "poster.jpg"}
	if fmt.Sprint(rels) != fmt.Sprint(want) {
		t.Fatalf("walk found %v, want %v", rels, want)
	}

	f, err := m.Open("Season 1/e2.mkv")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	got, err := io.ReadAll(f)
	if err != nil || !bytes.Equal(got, big) {
		t.Fatalf("read back %d bytes (err %v), want %d matching bytes", len(got), err, len(big))
	}
	// SeekEnd is what http.ServeContent uses to size responses — the raw
	// NFS client refuses it; the netfs wrapper must translate it.
	if pos, err := f.Seek(-16, io.SeekEnd); err != nil || pos != int64(len(big)-16) {
		t.Fatalf("SeekEnd: pos %d err %v", pos, err)
	}
	tail, err := io.ReadAll(f)
	if err != nil || !bytes.Equal(tail, big[len(big)-16:]) {
		t.Fatalf("tail after SeekEnd = %q (err %v)", tail, err)
	}
}

func TestNFS_BridgeServesRangeRequests(t *testing.T) {
	dir := t.TempDir()
	content := bytes.Repeat([]byte("chanarr"), 1000)
	writeTree(t, dir, map[string][]byte{"Media/Show/e1.mkv": content})
	host := startNFSServer(t, dir)

	mgr := NewManager(nil)
	if err := mgr.Start(); err != nil {
		t.Fatal(err)
	}
	// No pre-mount: InputTarget resolves the export on its own (shallowest
	// -first + Lookup-verify), so the bridge works even for an item whose
	// folder was never separately mounted this session.
	url, err := mgr.InputTarget("nfs://" + host + "/Media/Show/e1.mkv")
	if err != nil {
		t.Fatalf("InputTarget: %v", err)
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !bytes.Equal(body, content) {
		t.Fatalf("GET %s: status %d, %d bytes", url, resp.StatusCode, len(body))
	}
	if resp.Header.Get("Accept-Ranges") != "bytes" {
		t.Fatalf("bridge must advertise Accept-Ranges (ffmpeg seeks with Range requests), got %q", resp.Header.Get("Accept-Ranges"))
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Range", "bytes=6000-")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	part, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusPartialContent || !bytes.Equal(part, content[6000:]) {
		t.Fatalf("Range request: status %d, %d bytes, want 206 with %d bytes", resp2.StatusCode, len(part), len(content)-6000)
	}

	// Unregistered ids must 404 — the bridge serves only what streaming
	// or scanning explicitly registered, never arbitrary paths.
	resp3, err := http.Get(url[:len(url)-32] + "00000000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("unregistered id: status %d, want 404", resp3.StatusCode)
	}
}
