package core

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttachmentDeletionReferencesTrashForceBackupAndReimport(t *testing.T) {
	s, w := fixture(t)
	add := func(name string) Attachment {
		t.Helper()
		a, e := s.ImportAttachment(name, base64.StdEncoding.EncodeToString([]byte(name)), false)
		if e != nil {
			t.Fatal(e)
		}
		return a
	}
	live, trash, unused := add("引用.png"), add("回收.png"), add("空闲.png")
	a, b := mustNote(t, s, "引用来源"), mustNote(t, s, "回收来源")
	a = graphTestAppend(t, s, a, "\n![图](smartnote-asset://"+live.ID+")")
	b = graphTestAppend(t, s, b, "\n![图](smartnote-asset://"+trash.ID+")")
	if e := s.DeleteNote(b.ID, b.Hash); e != nil {
		t.Fatal(e)
	}
	usage, e := s.AttachmentUsage(w.ID)
	if e != nil {
		t.Fatal(e)
	}
	counts := map[string]AttachmentUsage{}
	for _, u := range usage {
		counts[u.ID] = u
	}
	if counts[live.ID].Notes != 1 || counts[trash.ID].Trash != 1 || counts[unused.ID].Notes != 0 {
		t.Fatal(counts)
	}
	result, e := s.DeleteAttachments(DeleteAttachmentsInput{WorkspaceID: w.ID, IDs: []string{live.ID, trash.ID, unused.ID}})
	if e != nil || len(result.Deleted) != 1 || result.Deleted[0] != unused.ID || len(result.Skipped) != 2 {
		t.Fatal(result, e)
	}
	if _, e = s.AttachmentData(unused.ID); e == nil {
		t.Fatal("deleted original readable")
	}
	if _, e = os.Stat(filepath.Join(s.data(w), "attachments", unused.ID)); !os.IsNotExist(e) {
		t.Fatal("original not deleted", e)
	}
	fresh := add("空闲.png")
	if fresh.ID == unused.ID {
		t.Fatal("dedup reused deleted identity")
	}
	before := workspaceFile(t, w.Path, a.Path)
	result, e = s.DeleteAttachments(DeleteAttachmentsInput{WorkspaceID: w.ID, IDs: []string{live.ID}, Force: true})
	if e != nil || len(result.Deleted) != 1 || workspaceFile(t, w.Path, a.Path) != before {
		t.Fatal("forced delete changed source note", result, e)
	}
	if _, e = s.DeleteAttachments(DeleteAttachmentsInput{WorkspaceID: "other", IDs: []string{fresh.ID}, Force: true}); e == nil {
		t.Fatal("wrong workspace accepted")
	}
	if _, e = s.AttachmentData(fresh.ID); e != nil {
		t.Fatal("wrong workspace deleted asset", e)
	}
	if e = validateMetadata(s.data(w)); e != nil {
		t.Fatal(e)
	}
	files, e := s.syncSnapshot(w)
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := files["assets/"+unused.ID]; ok {
		t.Fatal("deleted original uploaded")
	}
	backup := filepath.Join(t.TempDir(), "attachments.zip")
	if e = s.Export(backup, true); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Import(backup, filepath.Join(t.TempDir(), "restored")); e != nil {
		t.Fatal(e)
	}
	snapshot, e := s.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	for _, a := range snapshot.Attachments {
		if a.ID == unused.ID || a.ID == live.ID || a.Deleted {
			t.Fatal("deleted attachment restored", a)
		}
	}
	if len(snapshot.Attachments) != 2 {
		t.Fatal(snapshot.Attachments)
	}
	if e = s.Export(filepath.Join(t.TempDir(), "standard.zip"), false); e != nil {
		t.Fatal("standard export cannot handle tombstones", e)
	}
}

func TestAttachmentTombstoneWinsConcurrentImportMerge(t *testing.T) {
	s, w := fixture(t)
	a, e := s.ImportAttachment("a.txt", base64.StdEncoding.EncodeToString([]byte("a")), false)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(s.data(w), "attachments-manifest.json")
	old, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DeleteAttachments(DeleteAttachmentsInput{WorkspaceID: w.ID, IDs: []string{a.ID}}); e != nil {
		t.Fatal(e)
	}
	deleted, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2][]byte{{deleted, old}, {old, deleted}} {
		merged, ok := mergeAttachmentManifests(pair[0], pair[1])
		if !ok {
			t.Fatal("merge failed")
		}
		var list []Attachment
		if json.Unmarshal(merged, &list) != nil || len(list) != 1 || !list[0].Deleted {
			t.Fatal(string(merged))
		}
	}
	// A retained original after interrupted physical cleanup is never reuploaded.
	if e = os.WriteFile(filepath.Join(s.data(w), "attachments", a.ID), []byte("a"), 0600); e != nil {
		t.Fatal(e)
	}
	files, e := s.syncSnapshot(w)
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := files["assets/"+a.ID]; ok {
		t.Fatal("tombstoned orphan reuploaded")
	}
}

func TestAttachmentCleanupRechecksNewAndRemoteReferences(t *testing.T) {
	s, w := fixture(t)
	a, e := s.ImportAttachment("a.txt", base64.StdEncoding.EncodeToString([]byte("a")), false)
	if e != nil {
		t.Fatal(e)
	}
	n := mustNote(t, s, "新引用")
	if _, e = s.AttachmentUsage(w.ID); e != nil {
		t.Fatal(e)
	}
	graphTestAppend(t, s, n, "\n[附件](smartnote-asset://"+a.ID+")")
	result, e := s.DeleteAttachments(DeleteAttachmentsInput{WorkspaceID: w.ID, IDs: []string{a.ID}})
	if e != nil || len(result.Deleted) != 0 || !strings.Contains(result.Skipped[a.ID], "1篇笔记") {
		t.Fatal(result, e)
	}
	items := []Attachment{a, {ID: "remote", Name: "remote.png", RemoteOnly: true, RemoteURL: "https://cdn.example.test/remote.png"}}
	if e = writeJSON(filepath.Join(s.data(w), "attachments-manifest.json"), items); e != nil {
		t.Fatal(e)
	}
	remoteNote := mustNote(t, s, "远程引用")
	graphTestAppend(t, s, remoteNote, "\n![图](https://cdn.example.test/remote.png)")
	result, e = s.DeleteAttachments(DeleteAttachmentsInput{WorkspaceID: w.ID, IDs: []string{"remote"}})
	if e != nil || len(result.Deleted) != 0 {
		t.Fatal(result, e)
	}
	result, e = s.DeleteAttachments(DeleteAttachmentsInput{WorkspaceID: w.ID, IDs: []string{"remote"}, Force: true})
	if e != nil || len(result.Deleted) != 1 {
		t.Fatal(result, e)
	}
}

func TestWebDAVAttachmentDeletionWithConcurrentImportAndGraphAnnotations(t *testing.T) {
	server := httptest.NewServer(&testDAV{files: map[string][]byte{}})
	defer server.Close()
	a, wa, ca := syncFixture(t, server.URL)
	b, wb, cb := syncFixture(t, server.URL)
	old, e := a.ImportAttachment("旧附件.txt", base64.StdEncoding.EncodeToString([]byte("old")), false)
	if e != nil {
		t.Fatal(e)
	}
	source, target := mustNote(t, a, "图谱来源"), mustNote(t, a, "图谱目标")
	if e = a.SetGraphRelation(GraphRelationInput{WorkspaceID: wa.ID, Source: source.ID, Target: target.ID, SourceHash: source.Hash, TargetHash: target.Hash, Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	if _, e = a.SaveGraphAnnotation(GraphAnnotationInput{WorkspaceID: wa.ID, NoteID: source.ID, Text: "跨设备备注"}); e != nil {
		t.Fatal(e)
	}
	syncRun(t, a, wa, ca)
	syncRun(t, b, wb, cb)
	if _, e = a.DeleteAttachments(DeleteAttachmentsInput{WorkspaceID: wa.ID, IDs: []string{old.ID}}); e != nil {
		t.Fatal(e)
	}
	fresh, e := b.ImportAttachment("新附件.txt", base64.StdEncoding.EncodeToString([]byte("new")), false)
	if e != nil {
		t.Fatal(e)
	}
	syncRun(t, a, wa, ca)
	syncRun(t, b, wb, cb)
	syncRun(t, a, wa, ca)
	syncRun(t, b, wb, cb)
	for _, device := range []struct {
		s *Service
		w Workspace
	}{{a, wa}, {b, wb}} {
		snapshot, e := device.s.Snapshot()
		if e != nil || len(snapshot.Attachments) != 1 || snapshot.Attachments[0].ID != fresh.ID {
			t.Fatal("deleted attachment resurrected or concurrent import lost", snapshot.Attachments, e)
		}
		if _, e = device.s.AttachmentData(old.ID); e == nil {
			t.Fatal("deleted attachment readable after sync")
		}
		annotations, e := device.s.GetGraphAnnotations(device.w.ID)
		if e != nil || annotations.Nodes[source.ID] != "跨设备备注" {
			t.Fatal("graph annotation lost during sync", annotations, e)
		}
	}
}
