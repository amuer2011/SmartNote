package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"
)

type memorySecrets struct {
	mu     sync.Mutex
	values map[string]string
}

func (m *memorySecrets) Get(k string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.values[k], nil
}
func (m *memorySecrets) Set(k, v string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[k] = v
	return nil
}
func (m *memorySecrets) Delete(k string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, k)
	return nil
}
func testSecrets(s *Service) { s.secrets = &memorySecrets{values: map[string]string{}} }

type testDAV struct {
	mu                sync.Mutex
	files             map[string][]byte
	ignoreConditions  bool
	beforeManifestPut func()
	fail              bool
}

func (d *testDAV) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if user, password, ok := r.BasicAuth(); !ok || user != "user" || password != "secret" {
		w.WriteHeader(401)
		return
	}
	if r.Method == "PUT" && strings.HasSuffix(r.URL.Path, "/workspace.json") && d.beforeManifestPut != nil {
		d.beforeManifestPut()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.fail {
		w.WriteHeader(503)
		return
	}
	data, exists := d.files[r.URL.Path]
	etag := `"` + digest(data) + `"`
	switch r.Method {
	case "MKCOL":
		w.WriteHeader(201)
	case "GET":
		if !exists {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("ETag", etag)
		w.Write(data)
	case "PUT":
		if !d.ignoreConditions && (r.Header.Get("If-None-Match") == "*" && exists || r.Header.Get("If-Match") != "" && r.Header.Get("If-Match") != etag) {
			w.WriteHeader(412)
			return
		}
		b, _ := io.ReadAll(r.Body)
		d.files[r.URL.Path] = b
		w.Header().Set("ETag", `"`+digest(b)+`"`)
		w.WriteHeader(201)
	case "DELETE":
		delete(d.files, r.URL.Path)
		w.WriteHeader(204)
	default:
		w.WriteHeader(405)
	}
}
func syncFixture(t *testing.T, url string) (*Service, Workspace, SyncConfig) {
	t.Helper()
	s, w := fixture(t)
	testSecrets(s)
	c, _ := s.GetSyncConfig(w.ID)
	c.Enabled = true
	c.Endpoint = url + "/dav"
	c.Username = "user"
	c.RemoteID = "shared"
	c, e := s.SaveSyncConfig(w.ID, c, "secret", false)
	if e != nil {
		t.Fatal(e)
	}
	return s, w, c
}
func syncRun(t *testing.T, s *Service, w Workspace, c SyncConfig) {
	t.Helper()
	if e := s.runSync(context.Background(), w, c); e != nil {
		t.Fatal(e)
	}
}
func writeNoteFile(t *testing.T, w Workspace, path, text string) {
	t.Helper()
	if e := atomicWrite(filepath.Join(w.Path, filepath.FromSlash(path)), []byte(text)); e != nil {
		t.Fatal(e)
	}
}
func readNoteFile(t *testing.T, w Workspace, path string) string {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(w.Path, filepath.FromSlash(path)))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestWebDAVTwoDeviceSyncConflictDeleteAndOffline(t *testing.T) {
	d := &testDAV{files: map[string][]byte{}}
	server := httptest.NewServer(d)
	defer server.Close()
	a, wa, ca := syncFixture(t, server.URL)
	b, wb, cb := syncFixture(t, server.URL)
	if _, e := a.TestSync(wa.ID); e != nil {
		t.Fatal(e)
	}
	n := mustNote(t, a, "同步中文笔记")
	syncRun(t, a, wa, ca)
	syncRun(t, b, wb, cb)
	if got := readNoteFile(t, wb, n.Path); got != n.Content {
		t.Fatal("initial content differs")
	}
	writeNoteFile(t, wa, n.Path, n.Content+"\nA 修改")
	syncRun(t, a, wa, ca)
	syncRun(t, b, wb, cb)
	base := readNoteFile(t, wb, n.Path)
	if !strings.Contains(base, "A 修改") {
		t.Fatal("single side edit missing")
	}
	writeNoteFile(t, wa, n.Path, base+"\n本机A")
	writeNoteFile(t, wb, n.Path, base+"\n本机B")
	syncRun(t, a, wa, ca)
	syncRun(t, b, wb, cb)
	x, e := b.ListSyncConflicts(wb.ID)
	if e != nil || len(x) != 1 {
		t.Fatalf("conflicts=%v err=%v", x, e)
	}
	p, e := b.ReadSyncConflict(wb.ID, x[0].ID)
	if e != nil || !strings.Contains(p.Local, "本机B") || !strings.Contains(p.Remote, "本机A") {
		t.Fatalf("bad conflict preview: %+v %v", p, e)
	}
	if e = b.ResolveSyncConflict(SyncResolution{WorkspaceID: wb.ID, ID: x[0].ID, Choice: "merge", Merged: base + "\n本机A\n本机B"}); e != nil {
		t.Fatal(e)
	}
	syncRun(t, b, wb, cb)
	syncRun(t, a, wa, ca)
	if !strings.Contains(readNoteFile(t, wa, n.Path), "本机B") {
		t.Fatal("merge not synced")
	}
	d.mu.Lock()
	d.fail = true
	d.mu.Unlock()
	if e = a.runSync(context.Background(), wa, ca); e == nil {
		t.Fatal("offline succeeded")
	}
	d.mu.Lock()
	d.fail = false
	d.mu.Unlock()
	syncRun(t, a, wa, ca)
	if e = os.Remove(filepath.Join(wa.Path, filepath.FromSlash(n.Path))); e != nil {
		t.Fatal(e)
	}
	syncRun(t, a, wa, ca)
	syncRun(t, b, wb, cb)
	if _, e = os.Stat(filepath.Join(wb.Path, filepath.FromSlash(n.Path))); !os.IsNotExist(e) {
		t.Fatal("delete not propagated")
	}
	// A new/long-offline device cannot resurrect a tombstoned file silently.
	c, wc, cc := syncFixture(t, server.URL)
	writeNoteFile(t, wc, n.Path, n.Content)
	syncRun(t, c, wc, cc)
	cx, _ := c.ListSyncConflicts(wc.ID)
	if len(cx) != 1 || !cx[0].RemoteDeleted {
		t.Fatal("stale device resurrected deletion")
	}
	if e = c.ResolveSyncConflict(SyncResolution{WorkspaceID: wc.ID, ID: cx[0].ID, Choice: "remote"}); e != nil {
		t.Fatal(e)
	}
	syncRun(t, c, wc, cc)
}
func TestWebDAVAttachmentsAndMetadata(t *testing.T) {
	d := &testDAV{files: map[string][]byte{}}
	server := httptest.NewServer(d)
	defer server.Close()
	a, wa, ca := syncFixture(t, server.URL)
	b, wb, cb := syncFixture(t, server.URL)
	n := mustNote(t, a, "带附件")
	att, e := a.ImportAttachment("测试.txt", base64.StdEncoding.EncodeToString([]byte("附件中文")), false)
	if e != nil {
		t.Fatal(e)
	}
	_, e = a.AddBookmark(BookmarkInput{Kind: "note", WorkspaceID: wa.ID, NoteID: n.ID, Hash: n.Hash})
	if e != nil {
		t.Fatal(e)
	}
	syncRun(t, a, wa, ca)
	syncRun(t, b, wb, cb)
	items, e := b.GetBookmarks(wb.ID)
	if e != nil || len(items) != 1 || items[0].NoteID != n.ID {
		t.Fatalf("bookmarks %v %v", items, e)
	}
	got, e := os.ReadFile(filepath.Join(b.data(wb), "attachments", att.ID))
	if e != nil || string(got) != "附件中文" {
		t.Fatalf("attachment %s %v", got, e)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, value := range d.files {
		if bytes.Contains(value, []byte(wa.Path)) || bytes.Contains(value, []byte("\"secret\"")) {
			t.Fatal("credentials/local path leaked")
		}
	}
}
func TestWebDAVBackupAndConditionalSafety(t *testing.T) {
	d := &testDAV{files: map[string][]byte{}}
	server := httptest.NewServer(d)
	defer server.Close()
	a, wa, ca := syncFixture(t, server.URL)
	ca.Mode = "backup"
	ca, e := a.SaveSyncConfig(wa.ID, ca, "", false)
	if e != nil {
		t.Fatal(e)
	}
	n := mustNote(t, a, "备份")
	syncRun(t, a, wa, ca)
	if e = os.Remove(filepath.Join(wa.Path, filepath.FromSlash(n.Path))); e != nil {
		t.Fatal(e)
	}
	syncRun(t, a, wa, ca)
	if _, e = os.Stat(filepath.Join(wa.Path, filepath.FromSlash(n.Path))); !os.IsNotExist(e) {
		t.Fatal("backup downloaded content")
	}
	b, wb, cb := syncFixture(t, server.URL)
	syncRun(t, b, wb, cb)
	if readNoteFile(t, wb, n.Path) != n.Content {
		t.Fatal("backup propagated delete")
	}
	d.mu.Lock()
	d.ignoreConditions = true
	d.mu.Unlock()
	if _, e = a.TestSync(wa.ID); e == nil {
		t.Fatal("unsafe server accepted")
	}
	if e = a.runSync(context.Background(), wa, ca); e == nil {
		t.Fatal("unsafe server sync accepted")
	}
}
func TestWebDAVManifestRaceAndConcurrentLocalEdit(t *testing.T) {
	d := &testDAV{files: map[string][]byte{}}
	server := httptest.NewServer(d)
	defer server.Close()
	a, wa, ca := syncFixture(t, server.URL)
	n := mustNote(t, a, "并发")
	d.beforeManifestPut = func() { writeNoteFile(t, wa, n.Path, n.Content+"\n网络期间编辑") }
	if e := a.runSync(context.Background(), wa, ca); e == nil || !strings.Contains(e.Error(), "本地文件发生变化") {
		t.Fatalf("stale edit accepted: %v", e)
	}
	if !strings.Contains(readNoteFile(t, wa, n.Path), "网络期间编辑") {
		t.Fatal("new edit lost")
	}
	d.beforeManifestPut = nil
	syncRun(t, a, wa, ca)
	x, _ := a.ListSyncConflicts(wa.ID)
	if len(x) != 1 {
		t.Fatalf("expected preserved conflict %v", x)
	}
	// A manifest ETag change rejects publication while preserving local content.
	b, wb, cb := syncFixture(t, server.URL)
	syncRun(t, b, wb, cb)
	_ = wb
	_ = cb
	m, _, _ := func() (syncManifest, string, error) {
		cl, _ := newDAV(ca, "secret")
		return cl.manifest(context.Background())
	}()
	m.Files["notes/另一台.md"] = syncFile{Hash: digest([]byte("other"))}
	writeNoteFile(t, wa, "新笔记.md", "# 新笔记")
	d.beforeManifestPut = func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		raw, _ := json.Marshal(m)
		d.files["/dav/shared/workspace.json"] = raw
	}
	if e := a.runSync(context.Background(), wa, ca); e == nil || !strings.Contains(e.Error(), "另一台设备") {
		t.Fatalf("manifest race not rejected: %v", e)
	}
}
func TestExternalSecretsAndDestinations(t *testing.T) {
	s, w := fixture(t)
	testSecrets(s)
	c, _ := s.GetLLMConfig()
	c.Enabled = true
	c.Endpoint = "http://127.0.0.1:1234/v1"
	c.Model = "test"
	c, e := s.SaveLLMConfig(c, "private-api-key", false)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(s.root, "llm.json"))
	if bytes.Contains(b, []byte("private-api-key")) {
		t.Fatal("plain secret")
	}
	if !c.HasSecret {
		t.Fatal("secret missing")
	}
	c.Endpoint = "http://127.0.0.1:4321/v1"
	c, e = s.SaveLLMConfig(c, "", false)
	if e != nil || c.HasSecret {
		t.Fatalf("key forwarded to new destination: %+v %v", c, e)
	}
	for _, raw := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com/?key=x", "file:///test"} {
		if _, e = externalURL(raw, false); e == nil {
			t.Fatalf("unsafe URL accepted %s", raw)
		}
	}
	if _, e = externalURL("https://example.com", true); e == nil {
		t.Fatal("remote ollama")
	}
	_ = w
}
func TestLLMSelectionSummaryCancellationAndOff(t *testing.T) {
	s, w := fixture(t)
	testSecrets(s)
	n := mustNote(t, s, "原文")
	n, e := s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + "\n需要润色的文字\n检索事实：测试数据。"})
	if e != nil {
		t.Fatal(e)
	}
	var mu sync.Mutex
	var sent string
	slow := false
	entered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		sent = string(body)
		wait := slow
		mu.Unlock()
		if wait {
			entered <- struct{}{}
			<-r.Context().Done()
			return
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path %s", r.URL.Path)
		}
		io.WriteString(rw, `{"choices":[{"message":{"content":"润色结果 [1]"}}]}`)
	}))
	defer server.Close()
	c, _ := s.GetLLMConfig()
	c.Endpoint = server.URL + "/v1"
	c.Model = "mock"
	c.Enabled = true
	c, e = s.SaveLLMConfig(c, "", false)
	if e != nil {
		t.Fatal(e)
	}
	in := LLMInput{RequestID: uid(), WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Text: "需要润色的文字", Action: "polish"}
	in.From = len(utf16.Encode([]rune(n.Content[:strings.Index(n.Content, in.Text)])))
	in.To = in.From + len(utf16.Encode([]rune(in.Text)))
	out, e := s.GenerateLLM(in)
	if e != nil || out.Original != in.Text {
		t.Fatalf("polish %+v %v", out, e)
	}
	if readNoteFile(t, w, n.Path) != n.Content {
		t.Fatal("model auto-wrote note")
	}
	mu.Lock()
	if strings.Contains(sent, "检索事实") {
		t.Fatal("unselected text leaked")
	}
	mu.Unlock()
	in.RequestID = uid()
	in.Action = "summary"
	in.Query = "测试事实是什么"
	in.Sources = []LLMSource{{NoteID: n.ID, Hash: n.Hash, Line: 0}}
	out, e = s.GenerateLLM(in)
	if e != nil || len(out.Sources) != 1 || out.Sources[0].NoteID != n.ID {
		t.Fatalf("summary %+v %v", out, e)
	}
	in.Sources[0].Hash = "stale"
	if _, e = s.GenerateLLM(in); e == nil {
		t.Fatal("stale result sent")
	}
	in.Action = "polish"
	in.RequestID = uid()
	mu.Lock()
	slow = true
	mu.Unlock()
	done := make(chan error)
	go func() { _, e := s.GenerateLLM(in); done <- e }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request not started")
	}
	s.CancelLLM(in.RequestID)
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("cancellation ignored")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel blocked")
	}
	c.Enabled = false
	_, e = s.SaveLLMConfig(c, "", false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.GenerateLLM(in); e == nil {
		t.Fatal("disabled model requested")
	}
}
func TestLLMOllamaAndCitationValidation(t *testing.T) {
	s, w := fixture(t)
	testSecrets(s)
	n := mustNote(t, s, "摘要")
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("ollama path %s", r.URL.Path)
		}
		fmt.Fprint(rw, `{"message":{"content":"没有来源的回答"}}`)
	}))
	defer server.Close()
	c, _ := s.GetLLMConfig()
	c.Provider = "ollama"
	c.Endpoint = server.URL
	c.Model = "mock"
	c.Enabled = true
	if _, e := s.SaveLLMConfig(c, "", false); e != nil {
		t.Fatal(e)
	}
	if _, e := s.GenerateLLM(LLMInput{RequestID: uid(), WorkspaceID: w.ID, Action: "summary", Query: "摘要", Sources: []LLMSource{{NoteID: n.ID, Hash: n.Hash}}}); e == nil || !strings.Contains(e.Error(), "来源") {
		t.Fatalf("unattributed summary accepted: %v", e)
	}
}
