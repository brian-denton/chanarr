package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxLogoUploadMemory = 10 << 20 // 10MB — small poster images only

var allowedLogoExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
}

// handleUploadLogo accepts a multipart "logo" file, saved under logosDir
// named by channel id (so re-uploads simply overwrite). Manual upload
// always overrides whatever library.DetectLogo found (spec.md §8).
func (s *Server) handleUploadLogo(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	ch, err := s.store.Channel(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	if err := r.ParseMultipartForm(maxLogoUploadMemory); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart upload")
		return
	}
	file, header, err := r.FormFile("logo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing \"logo\" file field")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedLogoExtensions[ext] {
		writeError(w, http.StatusBadRequest, "unsupported image type "+ext+" (use jpg, png, or webp)")
		return
	}

	if err := os.MkdirAll(s.logosDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	destPath := filepath.Join(s.logosDir, fmt.Sprintf("%d%s", id, ext))

	// Clean up a previously uploaded logo under a different extension so
	// repeated re-uploads in different formats don't accumulate orphans.
	if ch.Logo != "" && strings.HasPrefix(ch.Logo, s.logosDir) && ch.Logo != destPath {
		os.Remove(ch.Logo)
	}

	dest, err := os.Create(destPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer dest.Close()
	if _, err := io.Copy(dest, file); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ch.Logo = destPath
	ch, err = s.store.SaveChannel(ch)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toChannelView(ch))
}

// handleServeLogo serves a channel's logo image — path is always either
// server-constructed (upload) or read back from a real directory listing
// (library.DetectLogo), never raw user input, so no traversal risk in
// passing it to http.ServeFile.
func (s *Server) handleServeLogo(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	ch, err := s.store.Channel(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if ch.Logo == "" {
		writeError(w, http.StatusNotFound, "channel has no logo")
		return
	}
	http.ServeFile(w, r, ch.Logo)
}
