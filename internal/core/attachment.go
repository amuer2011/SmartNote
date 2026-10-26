package core

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxAttachment = 100 << 20

const defaultPicGoURL = "http://127.0.0.1:36677"

func (s *Service) SetAttachmentPreferences(endpoint string, preferences AttachmentPreferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint != "" {
		u, err := url.Parse(endpoint)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return errors.New("PicGo 地址无效，请填写 HTTP Server 地址")
		}
	}
	previous := s.settings
	s.settings.PicGoURL, s.settings.Attachments = endpoint, preferences
	if err := s.persist(); err != nil {
		s.settings = previous
		return err
	}
	return nil
}

func validAttachmentRemote(a Attachment) bool {
	u, err := url.Parse(a.RemoteURL)
	return err == nil && u.Host != "" && (u.Scheme == "https" || u.Scheme == "http") && u.User == nil
}

func (s *Service) ImportAttachment(name, encoded string, paste bool) (Attachment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return Attachment{}, err
	}
	if len(encoded) > maxAttachment*4/3+4 {
		return Attachment{}, errors.New("附件不能超过 100 MB")
	}
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Attachment{}, err
	}
	if len(b) > maxAttachment {
		return Attachment{}, errors.New("附件不能超过 100 MB")
	}
	manifest := []Attachment{}
	manifestPath := filepath.Join(s.data(w), "attachments-manifest.json")
	if err = readJSON(manifestPath, &manifest); err != nil {
		return Attachment{}, err
	}
	preferences := s.settings.Attachments
	hash := digest(b)
	a := Attachment{ID: uid(), Name: filepath.Base(name), Mime: mime.TypeByExtension(filepath.Ext(name)), Size: int64(len(b)), SHA256: hash, Created: time.Now()}
	if a.Mime == "" {
		a.Mime = http.DetectContentType(b)
	}
	upload := paste && strings.HasPrefix(a.Mime, "image/") && preferences.PicGoEnabled
	index := -1
	if preferences.Deduplicate {
		// Prefer a retained original for local imports, and a known URL for pastes.
		for i, existing := range manifest {
			if existing.Deleted || existing.SHA256 != hash {
				continue
			}
			if existing.RemoteOnly && (!upload || preferences.KeepOriginal) {
				continue
			}
			if index < 0 || (upload && existing.RemoteURL != "") {
				index, a = i, existing
			}
			if upload && existing.RemoteURL != "" {
				break
			}
			if !upload && !existing.RemoteOnly {
				break
			}
		}
	}
	// A remote-only record cannot satisfy a request to retain an original.
	if index >= 0 && a.RemoteOnly && preferences.KeepOriginal {
		a = Attachment{ID: uid(), Name: filepath.Base(name), Mime: a.Mime, Size: int64(len(b)), SHA256: hash, Created: time.Now()}
		index = -1
	}
	fresh := index < 0
	path, err := safePath(filepath.Join(s.data(w), "attachments"), a.ID)
	if err != nil {
		return Attachment{}, err
	}
	if index < 0 {
		if err = atomicWrite(path, b); err != nil {
			return a, err
		}
		manifest = append(manifest, a)
		index = len(manifest) - 1
		if err = writeJSON(manifestPath, manifest); err != nil {
			return a, err
		}
	} else if !a.RemoteOnly {
		// A repeated paste can repair a missing/corrupt local original.
		if err = atomicWrite(path, b); err != nil {
			return a, err
		}
	}
	if upload && a.RemoteURL == "" {
		endpoint := strings.TrimSpace(s.settings.PicGoURL)
		if endpoint == "" {
			endpoint = defaultPicGoURL
		}
		// PicGo uses the filename extension to select an uploader/content type.
		// The permanent attachment ID intentionally has no extension.
		temp, tempErr := os.CreateTemp(s.data(w), ".picgo-*"+filepath.Ext(a.Name))
		if tempErr != nil {
			return a, nil
		}
		tempPath := temp.Name()
		defer os.Remove(tempPath)
		_, writeErr := temp.Write(b)
		closeErr := temp.Close()
		if writeErr == nil && closeErr == nil {
			remote, uploadErr := uploadPicGo(endpoint, tempPath)
			if uploadErr == nil {
				a.RemoteURL = remote
			}
		}
	}
	// Never remove an older retained original: existing notes may reference its ID.
	// Only a newly imported attachment may become remote-only.
	if fresh && upload && a.RemoteURL != "" && !preferences.KeepOriginal {
		a.RemoteOnly = true
		manifest[index] = a
		// Commit the remote-only manifest before removing the local original.
		if err = writeJSON(manifestPath, manifest); err != nil {
			return a, err
		}
		if err = os.Remove(path); err != nil {
			a.RemoteOnly = false
			manifest[index] = a
			if persistErr := writeJSON(manifestPath, manifest); persistErr != nil {
				return a, persistErr
			}
		}
		return a, nil
	}
	manifest[index] = a
	if err = writeJSON(manifestPath, manifest); err != nil {
		return a, err
	}
	return a, nil
}
func uploadPicGo(endpoint, path string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("PicGo 地址无效")
	}
	b, _ := json.Marshal(map[string]any{"list": []string{path}})
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Post(strings.TrimRight(endpoint, "/")+"/upload", "application/json", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("PicGo HTTP %d", resp.StatusCode)
	}
	var out struct {
		Success bool     `json:"success"`
		Result  []string `json:"result"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", err
	}
	if !out.Success || len(out.Result) == 0 {
		return "", errors.New("PicGo 上传失败")
	}
	remote, err := url.Parse(out.Result[0])
	if err != nil || remote.Host == "" || remote.User != nil || (remote.Scheme != "https" && remote.Scheme != "http") {
		return "", errors.New("PicGo 返回不安全地址")
	}
	return remote.String(), nil
}
func (s *Service) AttachmentData(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return "", err
	}
	items := []Attachment{}
	if err = readJSON(filepath.Join(s.data(w), "attachments-manifest.json"), &items); err != nil {
		return "", err
	}
	for _, a := range items {
		if a.ID == id && !a.Deleted {
			if a.RemoteOnly {
				if !validAttachmentRemote(a) {
					return "", errors.New("图床链接无效")
				}
				return a.RemoteURL, nil
			}
			path, err := safePath(filepath.Join(s.data(w), "attachments"), a.ID)
			if err != nil {
				return "", err
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			if digest(b) != a.SHA256 {
				return "", errors.New("附件哈希校验失败")
			}
			return "data:" + a.Mime + ";base64," + base64.StdEncoding.EncodeToString(b), nil
		}
	}
	return "", errors.New("附件不存在")
}
func (s *Service) CopyAttachment(id, dest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return err
	}
	items := []Attachment{}
	if err = readJSON(filepath.Join(s.data(w), "attachments-manifest.json"), &items); err != nil {
		return err
	}
	for _, a := range items {
		if a.ID == id && !a.Deleted {
			if a.RemoteOnly {
				return errors.New("此附件仅保留图床链接，请在浏览器中下载")
			}
			path, e := safePath(filepath.Join(s.data(w), "attachments"), a.ID)
			if e != nil {
				return e
			}
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			if digest(b) != a.SHA256 {
				return errors.New("附件校验失败")
			}
			return atomicWrite(dest, b)
		}
	}
	return errors.New("附件不存在")
}
