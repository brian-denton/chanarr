package netfs

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// Protocol identifies how a folder spec is reached.
const (
	ProtocolLocal = "local"
	ProtocolSMB   = "smb"
	ProtocolNFS   = "nfs"
)

// Location is a parsed folder or item spec. Specs are what get stored in
// the database (channel folders, epoch item paths): a plain filesystem
// path, or smb://host[:port]/share/dir..., or nfs://host[:port]/path...
// Credentials never appear in a spec — they live in the store, keyed by
// (protocol, host, share).
type Location struct {
	Protocol string
	Host     string // host[:port]; empty for local
	Share    string // SMB share name; empty for local and NFS
	// Dir is the path below the share (SMB, slash-separated, may be "")
	// or the full exported path (NFS, always starts with "/"). For local
	// it is the plain filesystem path.
	Dir string
}

// Remote reports whether the location needs a network client (and hence
// the ffmpeg bridge) rather than plain filesystem access.
func (l Location) Remote() bool { return l.Protocol != ProtocolLocal }

// Spec re-serializes the location with rel joined below it — the storable
// string form used for epoch item paths.
func (l Location) Spec(rel string) string {
	switch l.Protocol {
	case ProtocolSMB:
		return "smb://" + l.Host + "/" + l.Share + prefixSlash(path.Join(l.Dir, rel))
	case ProtocolNFS:
		return "nfs://" + l.Host + path.Join(l.Dir, rel)
	default:
		if rel == "" || rel == "." {
			return l.Dir
		}
		return strings.TrimSuffix(l.Dir, "/") + "/" + rel
	}
}

func prefixSlash(p string) string {
	if p == "" || p == "." {
		return ""
	}
	return "/" + p
}

// Parse classifies a folder/item spec. Anything without an smb:// or
// nfs:// scheme is a local filesystem path, preserving chanarr's original
// local-only behavior exactly.
func Parse(spec string) (Location, error) {
	switch {
	case strings.HasPrefix(spec, "smb://"):
		u, err := url.Parse(spec)
		if err != nil {
			return Location{}, fmt.Errorf("netfs: parse %s: %w", spec, err)
		}
		share, dir, _ := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/")
		if u.Host == "" || share == "" {
			return Location{}, fmt.Errorf("netfs: %s: need smb://host/share[/folder]", spec)
		}
		return Location{Protocol: ProtocolSMB, Host: u.Host, Share: share, Dir: strings.TrimSuffix(dir, "/")}, nil
	case strings.HasPrefix(spec, "nfs://"):
		u, err := url.Parse(spec)
		if err != nil {
			return Location{}, fmt.Errorf("netfs: parse %s: %w", spec, err)
		}
		if u.Host == "" || u.Path == "" || u.Path == "/" {
			return Location{}, fmt.Errorf("netfs: %s: need nfs://host/exported/folder", spec)
		}
		return Location{Protocol: ProtocolNFS, Host: u.Host, Dir: strings.TrimSuffix(u.Path, "/")}, nil
	default:
		return Location{Protocol: ProtocolLocal, Dir: spec}, nil
	}
}
