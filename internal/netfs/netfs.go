// Package netfs lets channel folders live on network shares (SMB, NFS) as
// well as the local filesystem. It has two halves:
//
//   - Mounts: a uniform fs.FS view over a folder spec ("/media/tv/Show",
//     "smb://nas/share/Show", "nfs://nas/export/Show") for scanning, plus
//     per-file open for reading bytes.
//   - The bridge: a loopback-only HTTP file server with Range support.
//     ffmpeg/ffprobe can't speak SMB or NFS themselves (neither protocol
//     is compiled into common builds), but they speak HTTP natively — so
//     remote files are handed to them as 127.0.0.1 bridge URLs instead of
//     paths. Range support is what makes -ss seeking work over it.
//
// Connections are deliberately not pooled: every mount and every bridge
// open dials fresh and closes when done. LAN dials are milliseconds, and
// it sidesteps both idle-session expiry and any question of whether the
// underlying clients tolerate concurrent use of one socket.
//
// SMB authenticates with optional username/password (empty = guest),
// looked up from the store by (host, share). NFS v3 has no password auth
// — access is granted by the server's export rules; chanarr presents
// AUTH_UNIX with its own process uid/gid.
package netfs

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sync"
)

// Credentials for an SMB share. Zero value means guest/anonymous.
type Credentials struct {
	Username string
	Password string
}

// CredentialLookup fetches stored credentials for a share. Returning zero
// Credentials (no error) means none stored — try guest.
type CredentialLookup func(protocol, host, share string) (Credentials, error)

// backend is one dialed connection scoped to a folder.
type backend interface {
	fs() fs.FS
	// open returns the file at rel (slash path below the folder) plus its
	// size — carried explicitly because the NFS client's Seek can't do
	// io.SeekEnd, which http.ServeContent relies on to size responses.
	open(rel string) (io.ReadSeekCloser, int64, error)
	close() error
}

// Manager turns folder specs into Mounts and runs the ffmpeg bridge.
type Manager struct {
	lookup CredentialLookup
	bridge *bridge

	// mu guards exports: memoized export roots per NFS host, so only the
	// first touch of a host pays the progressive mount-probe walk.
	mu      sync.Mutex
	exports map[string][]string
}

func NewManager(lookup CredentialLookup) *Manager {
	m := &Manager{lookup: lookup, exports: make(map[string][]string)}
	m.bridge = newBridge(m.openSpec)
	return m
}

// Start brings up the loopback bridge listener. Must be called once
// before any remote folder is streamed or probed.
func (m *Manager) Start() error { return m.bridge.start() }

// Mount dials (if remote) and returns a Mount for a folder spec. Callers
// own the result and must Close it when the scan/copy is done.
func (m *Manager) Mount(folder string) (*Mount, error) {
	loc, err := Parse(folder)
	if err != nil {
		return nil, err
	}
	b, err := m.dial(loc)
	if err != nil {
		return nil, err
	}
	return &Mount{loc: loc, mgr: m, b: b}, nil
}

func (m *Manager) dial(loc Location) (backend, error) {
	switch loc.Protocol {
	case ProtocolSMB:
		creds, err := m.lookup(ProtocolSMB, loc.Host, loc.Share)
		if err != nil {
			return nil, err
		}
		return dialSMB(loc, creds)
	case ProtocolNFS:
		return m.dialNFS(loc)
	default:
		return dialLocal(loc)
	}
}

// InputTarget converts a stored item path into what ffmpeg/ffprobe should
// be handed as input: local paths pass through untouched, remote specs
// become bridge URLs. This is internal/stream's resolver.
func (m *Manager) InputTarget(spec string) (string, error) {
	loc, err := Parse(spec)
	if err != nil {
		return "", err
	}
	if !loc.Remote() {
		return spec, nil
	}
	return m.bridge.register(spec)
}

// openSpec opens a full item spec for the bridge — the folder/file split
// doesn't matter here, so the whole path below the share is the rel.
func (m *Manager) openSpec(spec string) (io.ReadSeekCloser, int64, error) {
	loc, err := Parse(spec)
	if err != nil {
		return nil, 0, err
	}
	if !loc.Remote() {
		return nil, 0, fmt.Errorf("netfs: bridge refuses local path %s", spec)
	}
	fileLoc := loc
	rel := ""
	fileLoc.Dir, rel = splitLastSegment(loc.Dir)
	b, err := m.dial(fileLoc)
	if err != nil {
		return nil, 0, err
	}
	f, size, err := b.open(rel)
	if err != nil {
		b.close()
		return nil, 0, err
	}
	// The file handle owns the backend now — closing one closes both.
	return &backendFile{ReadSeekCloser: f, b: b}, size, nil
}

func splitLastSegment(p string) (dir, last string) {
	dir, last = filepath.ToSlash(p), ""
	if i := lastSlash(dir); i >= 0 {
		return dir[:i], dir[i+1:]
	}
	return "", dir
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

type backendFile struct {
	io.ReadSeekCloser
	b backend
}

func (f *backendFile) Close() error {
	err := f.ReadSeekCloser.Close()
	f.b.close()
	return err
}

// Mount is one folder opened for scanning. Not safe for concurrent use.
type Mount struct {
	loc Location
	mgr *Manager
	b   backend
}

// FS is the folder's tree, rooted at the folder itself, for fs.WalkDir.
func (m *Mount) FS() fs.FS { return m.b.fs() }

// Folder returns the original folder spec, for error messages.
func (m *Mount) Folder() string { return m.loc.Spec("") }

// Remote reports whether this mount crosses the network.
func (m *Mount) Remote() bool { return m.loc.Remote() }

// Spec returns the storable item path for a file at rel below the folder.
func (m *Mount) Spec(rel string) string { return m.loc.Spec(rel) }

// InputTarget returns what ffprobe should be handed for the file at rel:
// the plain path for local folders, a bridge URL for remote ones.
func (m *Mount) InputTarget(rel string) (string, error) {
	if !m.loc.Remote() {
		return filepath.Join(m.loc.Dir, filepath.FromSlash(rel)), nil
	}
	return m.mgr.bridge.register(m.loc.Spec(rel))
}

// Open reads one file below the folder — used to copy a detected logo off
// a remote share.
func (m *Mount) Open(rel string) (io.ReadSeekCloser, error) {
	f, _, err := m.b.open(rel)
	return f, err
}

func (m *Mount) Close() error { return m.b.close() }
