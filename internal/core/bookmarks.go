package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
)

type Bookmark struct {
	Kind    string `json:"kind,omitempty"`
	ID      string `json:"id"`
	NoteID  string `json:"noteId"`
	Label   string `json:"label"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Text    string `json:"text"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Created string `json:"created"`
}
type BookmarkInput struct {
	Kind        string `json:"kind,omitempty"`
	WorkspaceID string `json:"workspaceId"`
	NoteID      string `json:"noteId"`
	Hash        string `json:"hash"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Label       string `json:"label"`
}

func (s *Service) GetBookmarks(id string) ([]Bookmark, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.archiveWorkspace(id)
	if e != nil {
		return nil, e
	}
	items := []Bookmark{}
	e = readJSON(filepath.Join(s.data(w), "bookmarks.json"), &items)
	return items, e
}
func (s *Service) AddBookmark(input BookmarkInput) (Bookmark, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.archiveWorkspace(input.WorkspaceID)
	if e != nil {
		return Bookmark{}, e
	}
	n, e := s.archiveNote(w, input.NoteID, input.Hash)
	if e != nil {
		return Bookmark{}, e
	}
	kind := input.Kind
	if kind == "" {
		kind = "position"
	}
	if kind != "position" && kind != "note" {
		return Bookmark{}, errors.New("书签类型无效")
	}
	if kind == "note" {
		input.Line = 0
		input.Column = 0
	}
	lines := strings.Split(strings.ReplaceAll(n.Content, "\r\n", "\n"), "\n")
	if input.Line < 0 || input.Line >= len(lines) || input.Column < 0 || input.Column > len(utf16.Encode([]rune(lines[input.Line]))) {
		return Bookmark{}, errors.New("书签位置无效")
	}
	label := strings.TrimSpace(input.Label)
	if len([]rune(label)) > 200 {
		return Bookmark{}, errors.New("书签名称不能超过 200 字")
	}
	if label == "" {
		if kind == "note" {
			label = n.Title
		} else {
			label = strings.TrimSpace(lines[input.Line])
		}
		if label == "" {
			label = n.Title
		}
		r := []rune(label)
		if len(r) > 80 {
			label = string(r[:80])
		}
	}
	items := []Bookmark{}
	path := filepath.Join(s.data(w), "bookmarks.json")
	if e = readJSON(path, &items); e != nil {
		return Bookmark{}, e
	}
	for _, v := range items {
		if v.NoteID == n.ID && ((kind == "note" && v.Kind == "note") || (kind == "position" && v.Kind != "note" && v.Line == input.Line && v.Column == input.Column && v.Text == lines[input.Line])) {
			return v, nil
		}
	}
	raw := make([]byte, 16)
	if _, e = rand.Read(raw); e != nil {
		return Bookmark{}, e
	}
	b := Bookmark{Kind: kind, ID: hex.EncodeToString(raw), NoteID: n.ID, Label: label, Line: input.Line, Column: input.Column, Text: lines[input.Line], Created: time.Now().Format(time.RFC3339)}
	if input.Line > 0 {
		b.Before = lines[input.Line-1]
	}
	if input.Line+1 < len(lines) {
		b.After = lines[input.Line+1]
	}
	if kind == "note" {
		b.Text = ""
		b.Before = ""
		b.After = ""
	}
	items = append(items, b)
	return b, writeJSON(path, items)
}
func (s *Service) DeleteBookmark(workspaceID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.archiveWorkspace(workspaceID)
	if e != nil {
		return e
	}
	path := filepath.Join(s.data(w), "bookmarks.json")
	items := []Bookmark{}
	if e = readJSON(path, &items); e != nil {
		return e
	}
	for i, b := range items {
		if b.ID == id {
			return writeJSON(path, append(items[:i], items[i+1:]...))
		}
	}
	return errors.New("书签不存在")
}
