package core

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const maxTextPreview = 2 << 20

type AttachmentPreview struct {
	Kind    string `json:"kind"`
	Content string `json:"content"`
	Message string `json:"message"`
}

func (s *Service) PreviewAttachment(workspaceID, id string) (AttachmentPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return AttachmentPreview{}, err
	}
	items := []Attachment{}
	if err = readJSON(filepath.Join(s.data(w), "attachments-manifest.json"), &items); err != nil {
		return AttachmentPreview{}, err
	}
	for _, a := range items {
		if a.ID != id || a.Deleted {
			continue
		}
		if a.RemoteOnly {
			return AttachmentPreview{Kind: "file", Message: "此附件仅保留图床链接，可在浏览器中查看或下载。"}, nil
		}
		path, err := safePath(filepath.Join(s.data(w), "attachments"), a.ID)
		if err != nil {
			return AttachmentPreview{}, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return AttachmentPreview{}, err
		}
		if !info.Mode().IsRegular() {
			return AttachmentPreview{}, errors.New("附件不是普通文件")
		}
		fallback := AttachmentPreview{Kind: "file", Message: "此类型附件暂不支持内容预览，可下载后查看。"}
		if info.Size() > maxTextPreview {
			fallback.Message = "文件超过 2 MB 文本预览上限，请下载查看。"
			return fallback, nil
		}
		f, err := os.Open(path)
		if err != nil {
			return AttachmentPreview{}, err
		}
		b, err := io.ReadAll(io.LimitReader(f, maxTextPreview+1))
		f.Close()
		if err != nil {
			return AttachmentPreview{}, err
		}
		if len(b) > maxTextPreview {
			return fallback, nil
		}
		if digest(b) != a.SHA256 {
			return AttachmentPreview{}, errors.New("附件哈希校验失败")
		}
		mime := strings.ToLower(strings.Split(a.Mime, ";")[0])
		ext := strings.ToLower(filepath.Ext(a.Name))
		textType := strings.HasPrefix(mime, "text/") || strings.Contains(mime, "json") || strings.Contains(mime, "xml") || strings.Contains(mime, "javascript")
		switch ext {
		case ".txt", ".md", ".markdown", ".json", ".jsonl", ".log", ".yaml", ".yml", ".xml", ".csv", ".tsv", ".ini", ".conf", ".config", ".toml", ".sql", ".java", ".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".css", ".html", ".sh", ".bat", ".ps1", ".env", ".svg":
			textType = true
		}
		if !textType {
			return fallback, nil
		}
		if len(b) >= 2 && ((b[0] == 0xff && b[1] == 0xfe) || (b[0] == 0xfe && b[1] == 0xff)) {
			if len(b)%2 != 0 {
				return fallback, nil
			}
			little := b[0] == 0xff
			units := make([]uint16, (len(b)-2)/2)
			for i := range units {
				x, y := b[2+i*2], b[3+i*2]
				if little {
					units[i] = uint16(x) | uint16(y)<<8
				} else {
					units[i] = uint16(x)<<8 | uint16(y)
				}
			}
			b = []byte(string(utf16.Decode(units)))
		}
		b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
		if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
			fallback.Message = "当前文本编码不支持预览，请下载查看。"
			return fallback, nil
		}
		for _, r := range string(b) {
			if r < 32 && r != '\n' && r != '\r' && r != '\t' && r != '\f' {
				return fallback, nil
			}
		}
		return AttachmentPreview{Kind: "text", Content: string(b)}, nil
	}
	return AttachmentPreview{}, errors.New("附件不存在")
}
