package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type picGoTransport func(*http.Request) (*http.Response, error)

func (f picGoTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPicGoPreferencesUploadDedupAndLocalFallback(t *testing.T) {
	s, w := fixture(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/upload" {
			t.Errorf("unexpected upload %s %s", r.Method, r.URL)
		}
		var input struct {
			List []string `json:"list"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || len(input.List) != 1 {
			t.Errorf("invalid input: %+v %v", input, err)
			rw.WriteHeader(400)
			return
		}
		if filepath.Ext(input.List[0]) != ".png" {
			t.Error("PicGo lost image extension", input.List[0])
		}
		if data, err := os.ReadFile(input.List[0]); err != nil || string(data) != "image bytes" {
			t.Error("missing original upload", err)
		}
		fmt.Fprint(rw, `{"success":true,"result":["https://cdn.example.test/image.png"]}`)
	}))
	defer server.Close()
	prefs := AttachmentPreferences{PicGoEnabled: false, KeepOriginal: true, Deduplicate: true}
	if err := s.SetAttachmentPreferences(server.URL, prefs); err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte("image bytes"))
	local, err := s.ImportAttachment("screen.png", encoded, true)
	if err != nil || calls != 0 || local.RemoteURL != "" {
		t.Fatal(local, err, calls)
	}
	prefs.PicGoEnabled = true
	if err = s.SetAttachmentPreferences(server.URL, prefs); err != nil {
		t.Fatal(err)
	}
	uploaded, err := s.ImportAttachment("screen.png", encoded, true)
	if err != nil || calls != 1 || uploaded.ID != local.ID || uploaded.RemoteURL == "" {
		t.Fatal("local duplicate was not uploaded", uploaded, err, calls)
	}
	again, err := s.ImportAttachment("same.png", encoded, true)
	if err != nil || calls != 1 || again.ID != uploaded.ID {
		t.Fatal("duplicate URL was not reused", again, err, calls)
	}
	if data, err := s.AttachmentData(local.ID); err != nil || !strings.HasPrefix(data, "data:image/png;") {
		t.Fatal("retained original missing", err)
	}
	if files, _ := filepath.Glob(filepath.Join(s.data(w), ".picgo-*")); len(files) != 0 {
		t.Fatal("temporary upload files leaked", files)
	}
	prefs.Deduplicate = false
	if err = s.SetAttachmentPreferences(server.URL, prefs); err != nil {
		t.Fatal(err)
	}
	independent, err := s.ImportAttachment("same.png", encoded, true)
	if err != nil || independent.ID == local.ID || calls != 2 {
		t.Fatal("dedup switch ignored", independent, err, calls)
	}
	server.Close()
	prefs.KeepOriginal = false
	if err = s.SetAttachmentPreferences(server.URL, prefs); err != nil {
		t.Fatal(err)
	}
	failed, err := s.ImportAttachment("offline.png", encoded, true)
	if err != nil || failed.RemoteOnly || failed.RemoteURL != "" {
		t.Fatal("failed upload did not fall back", failed, err)
	}
	if _, err = s.AttachmentData(failed.ID); err != nil {
		t.Fatal("fallback lost original", err)
	}
}

func TestPicGoDefaultAndRemoteOnlyBackupSync(t *testing.T) {
	s, w := fixture(t)
	if got := s.Settings().Attachments; !got.PicGoEnabled || !got.KeepOriginal || !got.Deduplicate {
		t.Fatal("missing defaults", got)
	}
	// Intercept the default URL without contacting a real user's PicGo service.
	previous := http.DefaultTransport
	called := 0
	http.DefaultTransport = picGoTransport(func(r *http.Request) (*http.Response, error) {
		called++
		if r.URL.String() != "http://127.0.0.1:36677/upload" {
			t.Error("incorrect default URL", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: http.NoBody, Header: http.Header{}}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = previous })
	prefs := AttachmentPreferences{PicGoEnabled: true, KeepOriginal: false, Deduplicate: true}
	if err := s.SetAttachmentPreferences("", prefs); err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte("remote only image"))
	fallback, err := s.ImportAttachment("test.png", encoded, true)
	if err != nil || called != 1 || fallback.RemoteOnly {
		t.Fatal("default request/fallback failed", fallback, err, called)
	}
	http.DefaultTransport = picGoTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true,"result":["https://cdn.example.test/only.png"]}`)), Header: http.Header{}}, nil
	})
	// An existing local record is retained even when the setting is later disabled.
	retained, err := s.ImportAttachment("test.png", encoded, true)
	if err != nil || retained.RemoteOnly || retained.RemoteURL == "" {
		t.Fatal("old original was discarded", retained, err)
	}
	remote, err := s.ImportAttachment("new.png", base64.StdEncoding.EncodeToString([]byte("different image")), true)
	if err != nil || !remote.RemoteOnly || remote.RemoteURL == "" {
		t.Fatal("remote-only switch ignored", remote, err)
	}
	if _, err = os.Stat(filepath.Join(s.data(w), "attachments", remote.ID)); !os.IsNotExist(err) {
		t.Fatal("remote-only left original", err)
	}
	if got, err := s.AttachmentData(remote.ID); err != nil || got != remote.RemoteURL {
		t.Fatal("remote preview failed", got, err)
	}
	if err = validateMetadata(s.data(w)); err != nil {
		t.Fatal("remote-only metadata invalid", err)
	}
	if _, err = s.syncSnapshot(w); err != nil {
		t.Fatal("remote-only broke sync", err)
	}
	backup := filepath.Join(t.TempDir(), "backup.zip")
	if err = s.Export(backup, true); err != nil {
		t.Fatal("remote-only broke native backup", err)
	}
	standard := filepath.Join(t.TempDir(), "standard.zip")
	if err = s.Export(standard, false); err != nil {
		t.Fatal("remote-only broke standard export", err)
	}
	restored, err := s.Import(backup, filepath.Join(t.TempDir(), "remote-only-restored"))
	if err != nil {
		t.Fatal("remote-only restore failed", err)
	}
	if got, err := s.AttachmentData(remote.ID); err != nil || got != remote.RemoteURL {
		t.Fatal("restored remote URL lost", got, err)
	}
	files, err := s.syncSnapshot(restored)
	if err != nil {
		t.Fatal("restored remote-only sync failed", err)
	}
	if err = s.validateSyncData(restored, files); err != nil {
		t.Fatal("remote-only sync validation failed", err)
	}
	settings := Settings{Attachments: AttachmentPreferences{PicGoEnabled: true, KeepOriginal: true, Deduplicate: true}}
	if err = readJSON(filepath.Join(s.root, "settings.json"), &settings); err != nil || settings.Attachments != prefs {
		t.Fatal("preferences not persisted", settings, err)
	}
}
