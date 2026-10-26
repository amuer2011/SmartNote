package core

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestAttachmentTextPreviewAndFallback(t *testing.T) {
	s, w := fixture(t)
	cases := []struct {
		name          string
		data          []byte
		kind, content string
	}{
		{"配置.json", []byte(`{"message":"中文 <script>alert(1)</script>"}`), "text", `{"message":"中文 <script>alert(1)</script>"}`},
		{"utf8.txt", append([]byte{0xef, 0xbb, 0xbf}, []byte("正文")...), "text", "正文"},
		{"utf16.txt", []byte{0xff, 0xfe, 'A', 0, 'B', 0}, "text", "AB"},
		{"document.pdf", []byte("%PDF-1.0 document"), "file", ""},
		{"binary.txt", []byte{0, 1, 2, 3}, "file", ""},
		{"large.txt", bytes.Repeat([]byte("a"), maxTextPreview+1), "file", ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			a, err := s.ImportAttachment(test.name, base64.StdEncoding.EncodeToString(test.data), false)
			if err != nil {
				t.Fatal(err)
			}
			preview, err := s.PreviewAttachment(w.ID, a.ID)
			if err != nil || preview.Kind != test.kind || preview.Content != test.content {
				t.Fatal("unexpected preview", err, preview.Kind)
			}
			if _, err = s.PreviewAttachment("other-workspace", a.ID); err == nil {
				t.Fatal("wrong workspace allowed")
			}
		})
	}
	a, err := s.ImportAttachment("protected.json", base64.StdEncoding.EncodeToString([]byte(`{"original":true}`)), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(s.data(w), "attachments", a.ID), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PreviewAttachment(w.ID, a.ID); err == nil {
		t.Fatal("corrupted attachment previewed")
	}
	if _, err = s.PreviewAttachment(w.ID, "../settings.json"); err == nil {
		t.Fatal("unknown path accepted")
	}
}
