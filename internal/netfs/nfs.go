package netfs

import (
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/willscott/go-nfs-client/nfs"
	"github.com/willscott/go-nfs-client/nfs/rpc"
)

// dialNFS connects to an NFS v3 server and resolves which part of the
// spec's path is the server's export. The spec doesn't distinguish them
// ("nfs://nas/volume1/media/Shows" — is the export /volume1 or
// /volume1/media?), so chanarr probes shallowest-first: it tries to MOUNT
// each ancestor from "/" down, and for each one that mounts, checks that
// the remaining subpath actually resolves under it (Lookup) before
// accepting it. Shallowest-first plus that check is correct for both
// strict servers (which only let you mount the exact export root, and
// reject the shallower tries) and lenient ones (which return the export
// root regardless of the MOUNT path — deepest-first would then wrongly
// compute an empty subdir and lose the leading path segments). Successful
// export roots are memoized per host so only the first touch pays for it.
func (m *Manager) dialNFS(loc Location) (backend, error) {
	conn, export, err := m.connectNFS(loc.Host, loc.Dir)
	if err != nil {
		return nil, err
	}
	dir := strings.Trim(strings.TrimPrefix(loc.Dir, export), "/")
	return &nfsBackend{conn: conn, dir: dir}, nil
}

// subdirRel is the path of fullPath relative to a mounted export, or "."
// when they're the same directory.
func subdirRel(export, fullPath string) string {
	rel := strings.Trim(strings.TrimPrefix(fullPath, export), "/")
	if rel == "" {
		return "."
	}
	return rel
}

// nfsConn is one dialed mount+target pair. With an explicit port in the
// host (nfs://host:2049/...) everything shares a single socket — the
// portmapper is skipped entirely, which is also what lets tests dial an
// in-process NFS server on a random port.
type nfsConn struct {
	mount  *nfs.Mount
	target *nfs.Target
}

func (c *nfsConn) close() {
	c.target.Client.Close()
	if c.mount.Client != c.target.Client {
		c.mount.Client.Close()
	}
}

func (m *Manager) connectNFS(host, fullPath string) (*nfsConn, string, error) {
	var mount *nfs.Mount
	if h, port, err := net.SplitHostPort(host); err == nil {
		client, err := nfs.DialServiceAtPort(h, atoiPort(port))
		if err != nil {
			return nil, "", fmt.Errorf("netfs: nfs dial %s: %w", host, err)
		}
		mount = &nfs.Mount{Client: client} // Addr left empty: reuse this socket for NFS calls too
	} else {
		var err error
		mount, err = nfs.DialMount(host, time.Second)
		if err != nil {
			return nil, "", fmt.Errorf("netfs: nfs dial %s: %w", host, err)
		}
	}

	auth := rpc.NewAuthUnix("chanarr", uint32(os.Getuid()), uint32(os.Getgid())).Auth()
	var lastErr error
	for _, candidate := range m.exportCandidates(host, fullPath) {
		target, err := mount.Mount(candidate, auth)
		if err != nil {
			lastErr = err
			continue
		}
		// The MOUNT succeeded, but that alone doesn't prove this export is
		// the right root: a lenient server hands back its export root for
		// any MOUNT path. Confirm the intended subdirectory actually
		// resolves under it before committing.
		if info, _, err := target.Lookup(subdirRel(candidate, fullPath)); err == nil && info.IsDir() {
			m.memoizeExport(host, candidate)
			return &nfsConn{mount: mount, target: target}, candidate, nil
		} else if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("%s is not a directory", fullPath)
		}
		// Only close a target that dialed its own socket; when it shares
		// the mount client (explicit-port case) closing it would kill the
		// socket the next candidate needs.
		if target.Client != mount.Client {
			target.Client.Close()
		}
	}
	mount.Client.Close()
	return nil, "", fmt.Errorf("netfs: nfs mount %s%s: no mountable export found: %w", host, fullPath, lastErr)
}

// exportCandidates lists paths to try MOUNTing, shallowest first: memoized
// known exports that prefix the path (already verified good), then "/" and
// each ancestor down to the full path. Shallowest-first is deliberate —
// see dialNFS's doc comment.
func (m *Manager) exportCandidates(host, fullPath string) []string {
	var out []string
	m.mu.Lock()
	for _, e := range m.exports[host] {
		if e == "/" || fullPath == e || strings.HasPrefix(fullPath, e+"/") {
			out = append(out, e)
		}
	}
	m.mu.Unlock()

	var ancestors []string
	for p := path.Clean(fullPath); ; p = path.Dir(p) {
		ancestors = append(ancestors, p)
		if p == "/" {
			break
		}
	}
	// ancestors is deepest-first from the walk above; reverse to shallowest.
	for i := len(ancestors) - 1; i >= 0; i-- {
		out = append(out, ancestors[i])
	}
	return out
}

func (m *Manager) memoizeExport(host, export string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.exports[host] {
		if e == export {
			return
		}
	}
	m.exports[host] = append(m.exports[host], export)
}

func atoiPort(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

type nfsBackend struct {
	mu   sync.Mutex // the NFS client serializes per-call, but not compound ops like list+lookup
	conn *nfsConn
	dir  string // below the mounted export, slash-separated, "" = the export itself
}

func (b *nfsBackend) abs(name string) string {
	p := b.dir
	if name != "." && name != "" {
		p = path.Join(b.dir, name)
	}
	if p == "" {
		p = "."
	}
	return p
}

func (b *nfsBackend) fs() fs.FS { return &nfsFS{b: b} }

func (b *nfsBackend) open(rel string) (io.ReadSeekCloser, int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p := b.abs(rel)
	info, _, err := b.conn.target.Lookup(p)
	if err != nil {
		return nil, 0, err
	}
	f, err := b.conn.target.Open(p)
	if err != nil {
		return nil, 0, err
	}
	return &nfsSeekableFile{f: f, size: info.Size()}, info.Size(), nil
}

func (b *nfsBackend) close() error {
	b.conn.close()
	return nil
}

// nfsSeekableFile adds io.SeekEnd support, which the NFS client's own
// Seek refuses (it doesn't track file size) but http.ServeContent depends
// on to size responses. Size is captured at open time via Lookup.
type nfsSeekableFile struct {
	f    *nfs.File
	size int64
}

func (s *nfsSeekableFile) Read(p []byte) (int, error) { return s.f.Read(p) }
func (s *nfsSeekableFile) Close() error               { return s.f.Close() }

func (s *nfsSeekableFile) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekEnd {
		return s.f.Seek(s.size+offset, io.SeekStart)
	}
	return s.f.Seek(offset, whence)
}

// nfsFS adapts an nfsBackend to io/fs for scanning. fs.WalkDir only needs
// ReadDir plus a Stat of the root; Open is implemented for completeness.
type nfsFS struct {
	b *nfsBackend
}

func (f *nfsFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	f.b.mu.Lock()
	defer f.b.mu.Unlock()
	p := f.b.abs(name)
	info, _, err := f.b.conn.target.Lookup(p)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	if info.IsDir() {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("netfs: directory opens not supported over nfs")}
	}
	file, err := f.b.conn.target.Open(p)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return &nfsFSFile{f: file, info: info}, nil
}

func (f *nfsFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	f.b.mu.Lock()
	defer f.b.mu.Unlock()
	info, _, err := f.b.conn.target.Lookup(f.b.abs(name))
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}
	return info, nil
}

func (f *nfsFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	f.b.mu.Lock()
	defer f.b.mu.Unlock()
	entries, err := f.b.conn.target.ReadDirPlus(f.b.abs(name))
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}
	out := make([]fs.DirEntry, 0, len(entries))
	for _, e := range entries {
		if e.FileName == "." || e.FileName == ".." {
			continue
		}
		out = append(out, &nfsDirEntry{e: e})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

type nfsDirEntry struct {
	e *nfs.EntryPlus
}

func (d *nfsDirEntry) Name() string               { return d.e.FileName }
func (d *nfsDirEntry) IsDir() bool                { return d.e.IsDir() }
func (d *nfsDirEntry) Type() fs.FileMode          { return d.e.Mode().Type() }
func (d *nfsDirEntry) Info() (fs.FileInfo, error) { return d.e, nil }

type nfsFSFile struct {
	f    *nfs.File
	info fs.FileInfo
}

func (f *nfsFSFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *nfsFSFile) Read(p []byte) (int, error) { return f.f.Read(p) }
func (f *nfsFSFile) Close() error               { return f.f.Close() }
