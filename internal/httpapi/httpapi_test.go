package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"chanarr/internal/netfs"
	"chanarr/internal/store"
)

// newTestServer builds a Server against an in-memory store and a temp
// logos directory, cleaned up automatically.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewServer(st, filepath.Join(t.TempDir(), "logos"), netfs.NewManager(nil))
}

// mustWriteFile creates a file (and its parent dirs) with the given
// content, used to build fixture media folders.
func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newMediaFolder builds a temp folder containing fake, non-probeable
// ".mkv" files. library.Scan logs and skips files ffprobe can't read
// rather than erroring, so these are fine for tests that only care about
// scan/creation *succeeding*, not about specific durations.
func newMediaFolder(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		mustWriteFile(t, filepath.Join(dir, name), "not real media")
	}
	return dir
}

func newJSONRequest(method, target, body string) (rec *httptest.ResponseRecorder, req *http.Request) {
	req = httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return httptest.NewRecorder(), req
}
