package core

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func rebaseCopiedTree(stage, source, target string, notes []Note) error {
	return filepath.WalkDir(stage, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		from := source + "/" + filepath.ToSlash(rel)
		to := target + "/" + filepath.ToSlash(rel)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content, err := rebaseCopiedMarkdown(string(b), from, to, source, target, notes)
		if err != nil {
			return err
		}
		if content == string(b) {
			return nil
		}
		return atomicWrite(path, []byte(content))
	})
}

// Rebase links in the copy only: internal folder links follow the copied tree,
// while references outside it keep pointing to their existing workspace files.
func rebaseCopiedMarkdown(content, from, to, source, target string, notes []Note) (string, error) {
	note := parseNote(from, []byte(content), time.Now())
	edits := []textEdit{}
	for _, ref := range markdownLinks(content) {
		value := ""
		if ref.Wiki {
			linked := resolveNoteLink(note, ref, notes)
			if linked == nil {
				continue
			}
			path := mappedEntry(linked.Path, source, target)
			fragment := ""
			if index := strings.IndexByte(ref.Target, '#'); index >= 0 {
				fragment = ref.Target[index:]
			}
			if strings.EqualFold(filepath.Ext(path), ".md") {
				path = strings.TrimSuffix(path, filepath.Ext(path))
			}
			value = path + fragment
		} else {
			resolved, suffix, ok := localTarget(from, ref.Target)
			if !ok {
				continue
			}
			originalRelative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(from)), filepath.FromSlash(resolved))
			if err != nil {
				return "", err
			}
			resolved = mappedEntry(resolved, source, target)
			relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(to)), filepath.FromSlash(resolved))
			if err != nil {
				return "", err
			}
			if relative == originalRelative {
				continue
			}
			value = encodedPath(filepath.ToSlash(relative)) + suffix
		}
		if value != ref.Target {
			edits = append(edits, textEdit{Start: ref.Start, End: ref.End, Value: value})
		}
	}
	return applyEdits(content, edits), nil
}
