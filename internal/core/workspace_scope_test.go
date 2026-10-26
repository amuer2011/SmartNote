package core

import (
	"encoding/base64"
	"path/filepath"
	"testing"
)

// Identical titles and queries in separate directories must remain independent
// after repeated workspace switches, including persisted assistant context.
func TestWorkspaceSwitchScopesKnowledgeAndMetadata(t *testing.T) {
	s, first := fixture(t)
	second, err := s.AddWorkspace("第二空间", filepath.Join(t.TempDir(), "notes"))
	if err != nil {
		t.Fatal(err)
	}
	type scope struct {
		workspace Workspace
		notes     []Note
		asset     Attachment
		chat      AssistantConversation
	}
	scopes := []scope{{workspace: first}, {workspace: second}}
	for i := range scopes {
		entry := &scopes[i]
		if err := s.OpenWorkspace(entry.workspace.ID); err != nil {
			t.Fatal(err)
		}
		a, b := mustNote(t, s, "同名入口"), mustNote(t, s, "同名目标")
		a, err = s.SaveNote(SaveInput{ID: a.ID, Path: a.Path, Hash: a.Hash, Content: a.Content + "\n[[同名目标]]\n- [ ] 公共查询词：完成本空间工作\n"})
		if err != nil {
			t.Fatal(err)
		}
		entry.notes = []Note{a, b}
		entry.asset, err = s.ImportAttachment("同名附件.txt", base64.StdEncoding.EncodeToString([]byte(entry.workspace.ID)), false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.AddBookmark(BookmarkInput{WorkspaceID: entry.workspace.ID, NoteID: a.ID, Hash: a.Hash, Kind: "note", Label: "本空间收藏"}); err != nil {
			t.Fatal(err)
		}
		if _, err = s.SaveAssistantMemory(entry.workspace.ID, AssistantMemory{Text: entry.workspace.ID}); err != nil {
			t.Fatal(err)
		}
		entry.chat, err = s.SaveAssistantConversation(entry.workspace.ID, AssistantConversation{ID: uid(), Entries: []AssistantEntry{{Role: "user", Content: entry.workspace.ID}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, i := range []int{0, 1, 0, 1} {
		entry := scopes[i]
		if err = s.OpenWorkspace(entry.workspace.ID); err != nil {
			t.Fatal(err)
		}
		snapshot, err := s.Snapshot()
		if err != nil || snapshot.Settings.ActiveID != entry.workspace.ID || len(snapshot.Notes) != 2 || len(snapshot.Tasks) != 1 || len(snapshot.Attachments) != 1 {
			t.Fatalf("workspace snapshot: %+v, %v", snapshot, err)
		}
		for _, note := range snapshot.Notes {
			if note.ID != entry.notes[0].ID && note.ID != entry.notes[1].ID {
				t.Fatal("foreign note in snapshot", note.ID)
			}
		}
		if snapshot.Tasks[0].NoteID != entry.notes[0].ID || snapshot.Attachments[0].ID != entry.asset.ID {
			t.Fatal("foreign task or attachment")
		}
		results, err := s.Search("公共查询词")
		if err != nil || len(results) != 1 || results[0].Note.ID != entry.notes[0].ID {
			t.Fatalf("foreign search result: %+v, %v", results, err)
		}
		edges, err := s.Graph()
		if err != nil || len(edges) != 1 || edges[0].Source != entry.notes[0].ID || edges[0].Target != entry.notes[1].ID {
			t.Fatalf("foreign graph edge: %+v, %v", edges, err)
		}
		bookmarks, err := s.GetBookmarks(entry.workspace.ID)
		if err != nil || len(bookmarks) != 1 || bookmarks[0].NoteID != entry.notes[0].ID {
			t.Fatalf("foreign bookmark: %+v, %v", bookmarks, err)
		}
		memory, err := s.GetAssistantMemory(entry.workspace.ID)
		if err != nil || memory.Text != entry.workspace.ID {
			t.Fatalf("foreign assistant memory: %+v, %v", memory, err)
		}
		chats, err := s.ListAssistantConversations(entry.workspace.ID)
		if err != nil || len(chats) != 1 || chats[0].ID != entry.chat.ID {
			t.Fatalf("foreign assistant history: %+v, %v", chats, err)
		}
	}
}
