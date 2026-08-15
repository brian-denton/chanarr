package netfs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type localBackend struct {
	dir string
}

// dialLocal stats the folder up front so a bad path fails here with a
// clear message, not deep inside a walk.
func dialLocal(loc Location) (backend, error) {
	info, err := os.Stat(loc.Dir)
	if err != nil {
		return nil, fmt.Errorf("netfs: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("netfs: %s is not a directory", loc.Dir)
	}
	return &localBackend{dir: loc.Dir}, nil
}

func (b *localBackend) fs() fs.FS { return os.DirFS(b.dir) }

func (b *localBackend) open(rel string) (io.ReadSeekCloser, int64, error) {
	f, err := os.Open(filepath.Join(b.dir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	return f, info.Size(), nil
}

func (b *localBackend) close() error { return nil }
