package netfs

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

// bridge is the loopback HTTP file server that hands remote files to
// ffmpeg/ffprobe. It only ever serves specs that were explicitly
// registered, addressed by unguessable random ids — never raw paths — so
// nothing on the LAN (it's bound to 127.0.0.1 anyway) or a stray local
// process can use it to read arbitrary share contents.
type bridge struct {
	open func(spec string) (io.ReadSeekCloser, int64, error)

	mu     sync.Mutex
	byID   map[string]string
	bySpec map[string]string
	base   string // http://127.0.0.1:port, "" until start
}

func newBridge(open func(string) (io.ReadSeekCloser, int64, error)) *bridge {
	return &bridge{open: open, byID: make(map[string]string), bySpec: make(map[string]string)}
}

func (b *bridge) start() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("netfs: bridge listen: %w", err)
	}
	b.mu.Lock()
	b.base = "http://" + ln.Addr().String()
	b.mu.Unlock()
	go http.Serve(ln, b) //nolint:errcheck // runs for the process lifetime
	return nil
}

// register maps a remote spec to a stable bridge URL. Idempotent per
// spec; entries live for the process lifetime (bounded by library size).
func (b *bridge) register(spec string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.base == "" {
		return "", fmt.Errorf("netfs: bridge not started")
	}
	if id, ok := b.bySpec[spec]; ok {
		return b.base + "/f/" + id, nil
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("netfs: bridge id: %w", err)
	}
	id := hex.EncodeToString(buf)
	b.byID[id] = spec
	b.bySpec[spec] = id
	return b.base + "/f/" + id, nil
}

func (b *bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/f/")
	b.mu.Lock()
	spec, ok := b.byID[id]
	b.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, _, err := b.open(spec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer f.Close()
	// Zero modtime: ServeContent skips cache validators, which is right —
	// ffmpeg re-requests ranges within one playback, never revalidates.
	http.ServeContent(w, r, path.Base(spec), time.Time{}, f)
}
