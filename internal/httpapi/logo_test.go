package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"
)

func uploadLogo(t *testing.T, s *Server, channelID int64, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("logo", filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(content)
	w.Close()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/channels/%d/logo", channelID), &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	s.Mux().ServeHTTP(rec, req)
	return rec
}

func TestHandleUploadLogo(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	created := createChannel(t, s, dir, "1", "X", false)

	rec := uploadLogo(t, s, created.ID, "logo.png", []byte("fake png bytes"))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var view channelView
	json.Unmarshal(rec.Body.Bytes(), &view)
	if !view.HasLogo {
		t.Error("expected HasLogo = true after upload")
	}
}

func TestHandleUploadLogo_ManualOverridesAutoDetected(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	mustWriteFile(t, dir+"/poster.jpg", "auto-detected poster")
	created := createChannel(t, s, dir, "1", "X", false)
	if !created.HasLogo {
		t.Fatal("expected auto-detected logo at creation")
	}

	rec := uploadLogo(t, s, created.ID, "manual.png", []byte("manually uploaded"))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	logoRec := httptest.NewRecorder()
	logoReq := httptest.NewRequest("GET", fmt.Sprintf("/api/channels/%d/logo", created.ID), nil)
	s.Mux().ServeHTTP(logoRec, logoReq)
	if logoRec.Body.String() != "manually uploaded" {
		t.Errorf("served logo = %q, want the manually uploaded content", logoRec.Body.String())
	}
}

func TestHandleUploadLogo_RejectsUnsupportedType(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	created := createChannel(t, s, dir, "1", "X", false)

	rec := uploadLogo(t, s, created.ID, "logo.exe", []byte("not an image"))
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleUploadLogo_ChannelNotFound(t *testing.T) {
	s := newTestServer(t)
	rec := uploadLogo(t, s, 999, "logo.png", []byte("x"))
	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleServeLogo_NoneSet(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	created := createChannel(t, s, dir, "1", "X", false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/channels/%d/logo", created.ID), nil)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleUploadLogo_ReuploadCleansUpPreviousFile(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	created := createChannel(t, s, dir, "1", "X", false)

	uploadLogo(t, s, created.ID, "first.png", []byte("first"))
	firstPath := s.logosDir + fmt.Sprintf("/%d.png", created.ID)
	if _, err := os.Stat(firstPath); err != nil {
		t.Fatalf("expected first upload at %s: %v", firstPath, err)
	}

	uploadLogo(t, s, created.ID, "second.jpg", []byte("second"))
	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Errorf("expected the old .png to be cleaned up after re-uploading as .jpg, stat err = %v", err)
	}
}

func TestHandleDeleteChannel_CleansUpUploadedLogo(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	created := createChannel(t, s, dir, "1", "X", false)
	uploadLogo(t, s, created.ID, "logo.png", []byte("logo bytes"))

	logoPath := s.logosDir + fmt.Sprintf("/%d.png", created.ID)
	if _, err := os.Stat(logoPath); err != nil {
		t.Fatalf("expected logo file to exist: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/channels/%d", created.ID), nil)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	if _, err := os.Stat(logoPath); !os.IsNotExist(err) {
		t.Errorf("expected the uploaded logo to be removed on channel delete, stat err = %v", err)
	}
}

func TestHandleDeleteChannel_NeverDeletesAutoDetectedLogo(t *testing.T) {
	// An auto-detected logo lives in the user's own media folder — it must
	// survive channel deletion untouched.
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	posterPath := dir + "/poster.jpg"
	mustWriteFile(t, posterPath, "user's own poster")
	created := createChannel(t, s, dir, "1", "X", false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/channels/%d", created.ID), nil)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	if _, err := os.Stat(posterPath); err != nil {
		t.Errorf("expected the user's own poster.jpg to survive, stat err = %v", err)
	}
}
