package core

import (
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type importAssetOptions struct {
	keepFiles  bool
	vaultPaths bool
	warn       func(string)
}

func (s *Service) convertImportedAssets(root, data string) error {
	return s.convertImportedAssetsOptions(root, data, importAssetOptions{})
}

func (s *Service) convertImportedAssetsOptions(root, data string, options importAssetOptions) error {
	files := []string{}
	if options.vaultPaths {
		if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				rel, e := filepath.Rel(root, path)
				if e != nil {
					return e
				}
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		}); err != nil {
			return err
		}
	}
	resolve := func(notePath, target string) (string, error) {
		value := strings.ReplaceAll(target, "\\", "/")
		candidates := []string{filepath.Join(filepath.Dir(notePath), filepath.FromSlash(value))}
		if options.vaultPaths {
			if strings.HasPrefix(value, "/") {
				candidates = []string{filepath.Join(root, filepath.FromSlash(strings.TrimLeft(value, "/")))}
			} else {
				candidates = append(candidates, filepath.Join(root, filepath.FromSlash(value)))
			}
		}
		for _, candidate := range candidates {
			rel, e := filepath.Rel(root, candidate)
			if e != nil {
				continue
			}
			local, e := safePath(root, rel)
			if e != nil {
				continue
			}
			info, e := os.Stat(local)
			if e == nil && info.Mode().IsRegular() {
				return local, nil
			}
		}
		if options.vaultPaths && !strings.Contains(value, "..") {
			matches := []string{}
			for _, rel := range files {
				if strings.EqualFold(rel, value) || strings.HasSuffix(strings.ToLower(rel), "/"+strings.ToLower(value)) {
					matches = append(matches, rel)
				}
			}
			if len(matches) == 1 {
				return safePath(root, matches[0])
			}
			if len(matches) > 1 {
				return "", errors.New("存在多个同名附件，请补充具体目录")
			}
		}
		return "", errors.New("附件不存在或路径超出来源文件夹")
	}

	manifest := []Attachment{}
	hashes := map[string]Attachment{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var resourceErr error
		edits := []textEdit{}
		for _, ref := range markdownLinks(string(b)) {
			u, e := url.Parse(ref.Target)
			if e != nil {
				if options.warn != nil {
					rel, _ := filepath.Rel(root, path)
					options.warn(fmt.Sprintf("%s：保留无法解析的链接 %s", filepath.ToSlash(rel), ref.Target))
					continue
				}
				resourceErr = e
				break
			}
			if u.IsAbs() || u.Host != "" || strings.HasPrefix(ref.Target, "#") {
				continue
			}
			if strings.EqualFold(filepath.Ext(u.Path), ".md") {
				continue
			}
			if ref.Wiki && !ref.Image {
				// Obsidian uses the same wiki syntax for notes and non-image attachments.
				if !options.vaultPaths || filepath.Ext(u.Path) == "" {
					continue
				}
				if _, noteErr := resolve(path, u.Path+".md"); noteErr == nil {
					continue
				}
				if _, assetErr := resolve(path, u.Path); assetErr != nil {
					continue
				}
			}

			local, e := resolve(path, u.Path)
			if e != nil {
				if options.warn != nil {
					rel, _ := filepath.Rel(root, path)
					options.warn(fmt.Sprintf("%s：保留未转换的附件链接 %s（%v）", filepath.ToSlash(rel), ref.Target, e))
					continue
				}
				resourceErr = fmt.Errorf("引用附件缺失: %s: %w", ref.Target, e)
				break
			}
			content, e := os.ReadFile(local)
			if e != nil {
				resourceErr = fmt.Errorf("读取附件失败: %s: %w", ref.Target, e)
				break
			}

			hash := digest(content)
			a, ok := hashes[hash]
			if !ok {
				a = Attachment{ID: uid(), Name: filepath.Base(local), Size: int64(len(content)), SHA256: hash, Mime: mime.TypeByExtension(filepath.Ext(local)), Created: time.Now()}
				if a.Mime == "" {
					a.Mime = http.DetectContentType(content)
				}
				if e = atomicWrite(filepath.Join(data, "attachments", a.ID), content); e != nil {
					resourceErr = e
					break
				}
				hashes[hash] = a
				manifest = append(manifest, a)
			}
			value := "smartnote-asset://" + a.ID
			if ref.Wiki {
				label := strings.NewReplacer("[", "\\[", "]", "\\]").Replace(ref.Label)
				prefix := ""
				if ref.Image {
					prefix = "!"
				}
				edits = append(edits, textEdit{Start: ref.FullStart, End: ref.FullEnd, Value: prefix + "[" + label + "](" + value + ")"})
			} else {
				edits = append(edits, textEdit{Start: ref.Start, End: ref.End, Value: value})
			}
		}
		if resourceErr != nil {
			return resourceErr
		}
		changed := applyEdits(string(b), edits)
		if changed == string(b) {
			return nil
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if err = atomicWrite(path, []byte(changed)); err != nil {
			return err
		}
		if options.keepFiles {
			return os.Chtimes(path, info.ModTime(), info.ModTime())
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Preserve unreferenced files too: migration must not silently discard attachments.
	if err = filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		content, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		hash := digest(content)
		if _, exists := hashes[hash]; exists {
			return nil
		}
		a := Attachment{ID: uid(), Name: filepath.Base(path), Size: int64(len(content)), SHA256: hash, Mime: mime.TypeByExtension(filepath.Ext(path)), Created: time.Now()}
		if a.Mime == "" {
			a.Mime = http.DetectContentType(content)
		}
		if e = atomicWrite(filepath.Join(data, "attachments", a.ID), content); e != nil {
			return e
		}
		hashes[hash] = a
		manifest = append(manifest, a)
		return nil
	}); err != nil {
		return err
	}
	// Folder import retains the source directory layout, including resource files.
	if options.keepFiles {
		return writeJSON(filepath.Join(data, "attachments-manifest.json"), manifest)
	}
	// ZIP imports keep their existing attachment-store-only format.
	if err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && !strings.EqualFold(filepath.Ext(path), ".md") {
			return os.Remove(path)
		}
		return nil
	}); err != nil {
		return err
	}
	return writeJSON(filepath.Join(data, "attachments-manifest.json"), manifest)
}
