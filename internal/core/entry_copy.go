package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var copySuffix = regexp.MustCompile(`-副本(?:\([0-9]+\))?$`)

// CopyEntry retains the sibling-copy API; navigation uses PasteEntry instead.
func (s *Service) CopyEntry(source string) (EntryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.copyEntryLocked(source, filepath.ToSlash(filepath.Dir(filepath.FromSlash(source))))
}

// PasteEntry copies to an explicit folder without changing the source or IDs.
func (s *Service) PasteEntry(source, directory string) (EntryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.copyEntryLocked(source, directory)
}

func (s *Service) copyEntryLocked(source, directory string) (EntryResult, error) {
	w, err := s.active()
	if err != nil {
		return EntryResult{}, err
	}
	original, source, err := entryPath(w, source)
	if err != nil {
		return EntryResult{}, err
	}
	info, err := os.Lstat(original)
	if err != nil {
		return EntryResult{}, err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return EntryResult{}, errors.New("只能复制普通文件或文件夹")
	}
	destination := w.Path
	if directory != "" && directory != "." {
		destination, directory, err = entryPath(w, directory)
		if err != nil {
			return EntryResult{}, err
		}
		folder, e := os.Stat(destination)
		if e != nil {
			return EntryResult{}, e
		}
		if !folder.IsDir() {
			return EntryResult{}, errors.New("粘贴目标必须为文件夹")
		}
	}
	if info.IsDir() && withinEntry(directory, source) {
		return EntryResult{}, errors.New("不能将文件夹粘贴到自身或子目录")
	}
	if info.IsDir() {
		if err = checkedDirectory(original); err != nil {
			return EntryResult{}, err
		}
	}
	notes, err := s.scan(w)
	if err != nil {
		return EntryResult{}, err
	}
	stage, err := os.MkdirTemp(destination, ".smartnote-copy-*")
	if err != nil {
		return EntryResult{}, err
	}
	defer os.RemoveAll(stage)
	prepared := filepath.Join(stage, "content")
	if info.IsDir() {
		err = filepath.WalkDir(original, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, e := filepath.Rel(original, path)
			if e != nil {
				return e
			}
			target := filepath.Join(prepared, rel)
			if entry.IsDir() {
				return os.Mkdir(target, 0700)
			}
			return prepareEntryCopy(path, target)
		})
	} else {
		err = prepareEntryCopy(original, prepared)
	}
	if err != nil {
		return EntryResult{}, err
	}
	name, ext := filepath.Base(original), ""
	if !info.IsDir() {
		ext = filepath.Ext(name)
		name = strings.TrimSuffix(name, ext)
	}
	name = copySuffix.ReplaceAllString(name, "") + "-副本"
	for i := 0; i < 10000; i++ {
		suffix := ""
		if i > 0 {
			suffix = fmt.Sprintf("(%d)", i)
		}
		target := filepath.Join(destination, name+suffix+ext)
		rel, e := filepath.Rel(w.Path, target)
		if e != nil {
			return EntryResult{}, e
		}
		rel = filepath.ToSlash(rel)
		if _, e := os.Lstat(target); e == nil {
			continue
		} else if !errors.Is(e, os.ErrNotExist) {
			return EntryResult{}, e
		}
		if info.IsDir() {
			// Exclusive reservation never merges with an existing folder.
			err = os.Mkdir(target, 0700)
			if errors.Is(err, os.ErrExist) {
				continue
			}
			if err != nil {
				return EntryResult{}, err
			}
			e := rebaseCopiedTree(prepared, source, rel, notes)
			var entries []os.DirEntry
			if e == nil {
				entries, e = os.ReadDir(prepared)
			}
			if e == nil {
				for _, child := range entries {
					if e = os.Rename(filepath.Join(prepared, child.Name()), filepath.Join(target, child.Name())); e != nil {
						break
					}
				}
			}
			if e != nil {
				os.RemoveAll(target)
				return EntryResult{}, e
			}
		} else {
			if strings.EqualFold(ext, ".md") {
				var b []byte
				b, err = os.ReadFile(prepared)
				if err == nil {
					var content string
					content, err = rebaseCopiedMarkdown(string(b), source, rel, source, rel, notes)
					if err == nil {
						rebased := filepath.Join(stage, "rebased")
						err = atomicWrite(rebased, []byte(content))
						if err == nil {
							err = copyEntryBytes(rebased, target)
						}
					}
				}
			} else {
				err = copyEntryBytes(prepared, target)
			}
			if errors.Is(err, os.ErrExist) {
				continue
			}
			if err != nil {
				return EntryResult{}, err
			}
		}
		return EntryResult{Path: rel}, nil
	}
	return EntryResult{}, errors.New("副本数量过多，请先修改文件名称")
}

func prepareEntryCopy(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("文件夹包含链接或特殊文件，不能复制")
	}
	if !strings.EqualFold(filepath.Ext(source), ".md") {
		return copyEntryBytes(source, target)
	}
	if info.Size() > 32<<20 {
		return errors.New("笔记超过 32 MB，无法复制")
	}
	b, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	text, err := copyNoteIdentity(string(b))
	if err != nil {
		return fmt.Errorf("复制 %s：%w", filepath.Base(source), err)
	}
	return writeExclusive(target, []byte(text))
}

func copyEntryBytes(source, target string) (err error) {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		out.Close()
		if err != nil {
			os.Remove(target)
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}
	return out.Close()
}

func copyNoteIdentity(content string) (string, error) {
	newline := "\n"
	if strings.Contains(content, "\r\n") {
		newline = "\r\n"
	}
	meta := yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	body := content
	if strings.HasPrefix(content, "---"+newline) {
		lines := strings.SplitAfter(content, newline)
		end := -1
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				end = i
				break
			}
		}
		if end < 0 {
			return "", errors.New("Front Matter 未闭合，请先修复 YAML")
		}
		var doc yaml.Node
		if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "")), &doc); err != nil {
			return "", err
		}
		if len(doc.Content) > 0 {
			meta = *doc.Content[0]
		}
		if meta.Kind != yaml.MappingNode {
			return "", errors.New("Front Matter 必须为键值结构")
		}
		var check map[string]any
		if err := meta.Decode(&check); err != nil {
			return "", err
		}
		body = strings.Join(lines[end+1:], "")
	}
	id := uid()
	value := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id}
	found := false
	for i := 0; i+1 < len(meta.Content); i += 2 {
		if meta.Content[i].Value == "id" {
			meta.Content[i+1] = value
			found = true
			break
		}
	}
	if !found {
		meta.Content = append(meta.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "id"}, value)
	}
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&meta); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	result := "---" + newline + strings.ReplaceAll(buf.String(), "\n", newline) + "---" + newline + body
	if parseNote("copy.md", []byte(result), time.Now()).ID != id {
		return "", errors.New("无法创建独立笔记 ID")
	}
	return result, nil
}
