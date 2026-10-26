package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWebDAVMoveEditConflictAllResolutions(t *testing.T) {
	for _, choice := range []string{"local", "remote", "merge"} {
		t.Run(choice, func(t *testing.T) {
			d := &testDAV{files: map[string][]byte{}}
			server := httptest.NewServer(d)
			defer server.Close()
			a, wa, ca := syncFixture(t, server.URL)
			b, wb, cb := syncFixture(t, server.URL)
			n := mustNote(t, a, "移动与编辑")
			syncRun(t, a, wa, ca)
			syncRun(t, b, wb, cb)
			target := "已移动/目标.md"
			moved, e := a.MoveNote(n.ID, n.Hash, target)
			if e != nil {
				t.Fatal(e)
			}
			writeNoteFile(t, wb, n.Path, n.Content+"\n另一台编辑")
			syncRun(t, a, wa, ca)
			syncRun(t, b, wb, cb)
			x, _ := b.ListSyncConflicts(wb.ID)
			if len(x) != 1 || x[0].RemotePath != "notes/"+target {
				t.Fatalf("move not grouped %+v", x)
			}
			// Refreshing the same unresolved conflict must not duplicate note identities.
			syncRun(t, b, wb, cb)
			x, _ = b.ListSyncConflicts(wb.ID)
			merged := n.Content + "\n合并完成"
			if e = b.ResolveSyncConflict(SyncResolution{WorkspaceID: wb.ID, ID: x[0].ID, Choice: choice, Merged: merged}); e != nil {
				t.Fatal(e)
			}
			syncRun(t, b, wb, cb)
			syncRun(t, a, wa, ca)
			for _, service := range []*Service{a, b} {
				snap, e := service.Snapshot()
				if e != nil || len(snap.Notes) != 1 {
					t.Fatalf("duplicate identity %+v %v", snap.Notes, e)
				}
				got := snap.Notes[0]
				switch choice {
				case "local":
					if got.Path != n.Path || !strings.Contains(got.Content, "另一台编辑") {
						t.Fatalf("local not preserved %+v", got)
					}
				case "remote":
					if got.Path != target || got.Content != moved.Note.Content {
						t.Fatal("remote not preserved")
					}
				case "merge":
					if got.Path != target || got.Content != merged {
						t.Fatal("merge not preserved")
					}
				}
			}
		})
	}
}

func TestWebDAVBackgroundCancellationDoesNotBlockSettings(t *testing.T) {
	d := &testDAV{files: map[string][]byte{}}
	entered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "workspace.json") {
			entered <- struct{}{}
			<-r.Context().Done()
			return
		}
		d.ServeHTTP(w, r)
	}))
	defer server.Close()
	s, w, _ := syncFixture(t, server.URL)
	if _, e := s.StartSync(w.ID); e != nil {
		t.Fatal(e)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("background sync did not start")
	}
	responsive := make(chan struct{})
	go func() { _ = s.Settings(); close(responsive) }()
	select {
	case <-responsive:
	case <-time.After(time.Second):
		t.Fatal("network request blocked main settings")
	}
	s.CancelSync(w.ID)
	deadline := time.After(5 * time.Second)
	for {
		if s.SyncStatus(w.ID).State == "error" {
			break
		}
		select {
		case <-deadline:
			t.Fatal("sync cancellation did not finish")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
func TestWebDAVConcurrentAttachmentImportsMerge(t *testing.T) {
	d := &testDAV{files: map[string][]byte{}}
	server := httptest.NewServer(d)
	defer server.Close()
	a, wa, ca := syncFixture(t, server.URL)
	b, wb, cb := syncFixture(t, server.URL)
	_, e := a.ImportAttachment("原始.txt", base64.StdEncoding.EncodeToString([]byte("原始")), false)
	if e != nil {
		t.Fatal(e)
	}
	syncRun(t, a, wa, ca)
	syncRun(t, b, wb, cb)
	for i, service := range []*Service{a, b} {
		if _, e = service.ImportAttachment("新附件.txt", base64.StdEncoding.EncodeToString([]byte{byte(i)}), false); e != nil {
			t.Fatal(e)
		}
	}
	syncRun(t, a, wa, ca)
	syncRun(t, b, wb, cb)
	syncRun(t, a, wa, ca)
	for _, service := range []*Service{a, b} {
		snap, e := service.Snapshot()
		if e != nil || len(snap.Attachments) != 3 {
			t.Fatalf("attachment lost %+v %v", snap.Attachments, e)
		}
	}
}
func TestSyncJournalRecoveryAndExternalProtection(t *testing.T) {
	s, w := fixture(t)
	testSecrets(s)
	path := "notes/中断.md"
	before := []byte("# 原文")
	after := []byte("# 下载版本")
	operation := s.syncPath(w, "operations/"+uid())
	if e := atomicWrite(filepath.Join(operation, "before", filepath.FromSlash(path)), before); e != nil {
		t.Fatal(e)
	}
	j := syncJournal{State: "prepared", Changes: []syncChange{{Key: path, BeforeHash: digest(before), AfterHash: digest(after)}}, Before: syncLocalState{Baseline: map[string]string{path: digest(before)}}}
	if e := writeJSON(filepath.Join(operation, "journal.json"), j); e != nil {
		t.Fatal(e)
	}
	writeNoteFile(t, w, "中断.md", string(after))
	if e := s.recoverSyncs(); e != nil {
		t.Fatal(e)
	}
	if readNoteFile(t, w, "中断.md") != string(before) {
		t.Fatal("interrupted transaction not rolled back")
	}
	j.State = "prepared"
	if e := writeJSON(filepath.Join(operation, "journal.json"), j); e != nil {
		t.Fatal(e)
	}
	writeNoteFile(t, w, "中断.md", "# 外部新修改")
	if e := s.recoverSyncs(); e == nil {
		t.Fatal("external edit overwritten")
	}
	if readNoteFile(t, w, "中断.md") != "# 外部新修改" {
		t.Fatal("external edit lost")
	}
}
func TestWebDAVRejectsUnsafeManifestAndCorruptObject(t *testing.T) {
	d := &testDAV{files: map[string][]byte{}}
	server := httptest.NewServer(d)
	defer server.Close()
	s, w, c := syncFixture(t, server.URL)
	n := mustNote(t, s, "安全")
	syncRun(t, s, w, c)
	for _, bad := range []string{"notes/../../settings.json", "metadata/llm.json", "notes/C:/bad.md", "notes/.hidden.md", "notes/CON.md"} {
		m := syncManifest{Version: 1, WorkspaceID: c.RemoteID, Files: map[string]syncFile{bad: {Hash: digest([]byte("bad"))}}}
		raw, _ := json.Marshal(m)
		d.mu.Lock()
		d.files["/dav/shared/workspace.json"] = raw
		d.mu.Unlock()
		if e := s.runSync(context.Background(), w, c); e == nil {
			t.Fatalf("unsafe path accepted %s", bad)
		}
		if readNoteFile(t, w, n.Path) != n.Content {
			t.Fatal("local overwritten")
		}
	}
	h := digest([]byte("expected"))
	m := syncManifest{Version: 1, WorkspaceID: c.RemoteID, Files: map[string]syncFile{"notes/new.md": {Hash: h}}}
	raw, _ := json.Marshal(m)
	d.mu.Lock()
	d.files["/dav/shared/workspace.json"] = raw
	d.files["/dav/shared/objects/"+h] = []byte("corrupted")
	d.mu.Unlock()
	if e := s.runSync(context.Background(), w, c); e == nil {
		t.Fatal("corrupt object accepted")
	}
	if _, e := os.Stat(filepath.Join(w.Path, "new.md")); !os.IsNotExist(e) {
		t.Fatal("corrupted local file created")
	}
}
