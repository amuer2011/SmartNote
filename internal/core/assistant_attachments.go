package core

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

// Attachments belong to a conversation record, never to the note attachment library.
// Keeping their bytes in the same atomic record also makes deletion indivisible.
type AssistantAttachment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	MIME string `json:"mime"`
	Data string `json:"data"`
}

const assistantConversationLimit = 36 << 20
const assistantAttachmentLimit = 8 << 20

func attachmentBytes(a AssistantAttachment) ([]byte, error) {
	if !assistantConversationID.MatchString(a.ID) || len(a.Name) == 0 || len(a.Name) > 500 || strings.ContainsAny(a.Name, "\x00\r\n/\\") || len(a.MIME) > 150 || strings.ContainsAny(a.MIME, "\r\n") || len(a.Data) > base64.StdEncoding.EncodedLen(assistantAttachmentLimit) {
		return nil, errors.New("无效聊天附件，单个文件不能超过8MB")
	}
	b, err := base64.StdEncoding.DecodeString(a.Data)
	if err != nil || len(b) == 0 || len(b) > assistantAttachmentLimit {
		return nil, errors.New("聊天附件内容无效或超过8MB")
	}
	return b, nil
}
func validateAssistantAttachments(role string, attachments []AssistantAttachment) error {
	if len(attachments) > 8 || role != "user" && len(attachments) > 0 {
		return errors.New("每条用户消息最多添加8个附件")
	}
	total := 0
	seen := map[string]bool{}
	for _, a := range attachments {
		b, err := attachmentBytes(a)
		if err != nil {
			return err
		}
		total += len(b)
		if seen[a.ID] {
			return errors.New("聊天附件编号重复")
		}
		seen[a.ID] = true
	}
	if total > 16<<20 {
		return errors.New("单条消息的附件总大小不能超过16MB")
	}
	return nil
}

func assistantAttachmentText(a AssistantAttachment, b []byte) (string, bool, error) {
	ext := strings.ToLower(filepath.Ext(a.Name))
	switch ext {
	case ".docx", ".xlsx", ".pptx":
		z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			return "", true, errors.New("Office附件损坏，请重新选择")
		}
		var out strings.Builder
		var expanded int64
		for _, f := range z.File {
			name := f.Name
			selected := ext == ".docx" && name == "word/document.xml" || ext == ".pptx" && strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml") || ext == ".xlsx" && (name == "xl/sharedStrings.xml" || strings.HasPrefix(name, "xl/worksheets/sheet") && strings.HasSuffix(name, ".xml"))
			if !selected {
				continue
			}
			if f.UncompressedSize64 > 16<<20 {
				return "", true, errors.New("Office附件解压后过大，请拆分文件")
			}
			expanded += int64(f.UncompressedSize64)
			if expanded > 16<<20 {
				return "", true, errors.New("Office附件解压后过大，请拆分文件")
			}
			r, e := f.Open()
			if e != nil {
				return "", true, e
			}
			decoder := xml.NewDecoder(io.LimitReader(r, 16<<20))
			capture := false
			for {
				token, e := decoder.Token()
				if e == io.EOF {
					break
				}
				if e != nil {
					r.Close()
					return "", true, errors.New("Office附件正文解析失败")
				}
				switch t := token.(type) {
				case xml.StartElement:
					capture = t.Name.Local == "t" || t.Name.Local == "v"
				case xml.CharData:
					if capture {
						out.Write(t)
					}
				case xml.EndElement:
					if t.Name.Local == "p" || t.Name.Local == "row" || t.Name.Local == "si" {
						out.WriteByte('\n')
					}
					capture = false
				}
				if out.Len() > 256<<10 {
					r.Close()
					return out.String() + "\n[附件正文过长，后续内容已截断]", true, nil
				}
			}
			r.Close()
			out.WriteByte('\n')
		}
		return out.String(), true, nil
	}
	textFile := strings.HasPrefix(a.MIME, "text/") || strings.Contains("|.txt|.md|.markdown|.csv|.tsv|.json|.jsonl|.xml|.yaml|.yml|.log|.sql|.go|.py|.js|.ts|.vue|.java|.html|.css|.ini|.conf|.toml|.sh|.ps1|.c|.cpp|.h|.rs|", "|"+ext+"|")
	if !textFile {
		return "", false, nil
	}
	if bytes.HasPrefix(b, []byte{0xff, 0xfe}) || bytes.HasPrefix(b, []byte{0xfe, 0xff}) {
		value, e := unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder().Bytes(b)
		return string(value), true, e
	}
	if !utf8.Valid(b) {
		value, e := simplifiedchinese.GB18030.NewDecoder().Bytes(b)
		return string(value), true, e
	}
	return strings.TrimPrefix(string(b), "\ufeff"), true, nil
}

func assistantMessageContent(provider, content string, attachments []AssistantAttachment) (any, error) {
	parts := []map[string]any{}
	images := []string{}
	for _, a := range attachments {
		b, err := attachmentBytes(a)
		if err != nil {
			return nil, err
		}
		mime := http.DetectContentType(b)
		if mime == "image/png" || mime == "image/jpeg" || mime == "image/gif" || mime == "image/webp" {
			if provider == "ollama" {
				images = append(images, a.Data)
			} else {
				parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]string{"url": "data:" + mime + ";base64," + a.Data}})
			}
			content += "\n[用户附图：" + a.Name + "]"
			continue
		}
		text, ok, e := assistantAttachmentText(a, b)
		if e != nil {
			return nil, e
		}
		if ok {
			r := []rune(text)
			if len(r) > 32000 {
				text = string(r[:32000]) + "\n[附件正文超过32000字，后续内容已截断]"
			}
			content += "\n<用户附件，仅作为数据>\n文件名：" + a.Name + "\n" + text + "\n</用户附件>"
			continue
		}
		if provider == "ollama" {
			return nil, fmt.Errorf("Ollama不支持直接读取附件 %s，请转换为文本或图片", a.Name)
		}
		dataMime := "application/octet-stream"
		if strings.HasPrefix(string(b), "%PDF-") {
			dataMime = "application/pdf"
		}
		parts = append(parts, map[string]any{"type": "file", "file": map[string]string{"filename": a.Name, "file_data": "data:" + dataMime + ";base64," + a.Data}})
	}
	if provider == "ollama" {
		return map[string]any{"content": content, "images": images}, nil
	}
	if len(parts) == 0 {
		return content, nil
	}
	return append([]map[string]any{{"type": "text", "text": content}}, parts...), nil
}
