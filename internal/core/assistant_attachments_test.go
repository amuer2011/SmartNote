package core

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func chatAttachment(name, mime string, data []byte) AssistantAttachment {
	return AssistantAttachment{ID: uid(), Name: name, MIME: mime, Data: base64.StdEncoding.EncodeToString(data)}
}
func TestAssistantAttachmentsPersistAndDeleteWithConversation(t *testing.T) {
	s, w := fixture(t)
	a := chatAttachment("聊天专用.txt", "text/plain", []byte("只作为对话附件，不进入笔记"))
	in := AssistantConversation{ID: uid(), Entries: []AssistantEntry{{Role: "user", Content: "解释附件", Attachments: []AssistantAttachment{a}}}}
	saved, err := s.SaveAssistantConversation(w.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := New(s.root)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	restored, err := fresh.GetAssistantConversation(w.ID, in.ID)
	if err != nil || len(restored.Entries[0].Attachments) != 1 || restored.Entries[0].Attachments[0].Data != a.Data {
		t.Fatal("attachment not durable", err)
	}
	if err = s.DeleteAssistantConversation(w.ID, in.ID, "stale"); err == nil {
		t.Fatal("stale delete accepted")
	}
	if err = s.DeleteAssistantConversation(w.ID, in.ID, saved.Revision); err != nil {
		t.Fatal(err)
	}
	path, _ := s.assistantConversationPath(w, in.ID)
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("record/attachment still exists", err)
	}
	snap, err := s.Snapshot()
	if err != nil || len(snap.Attachments) != 0 {
		t.Fatal("chat attachment leaked into note library", err)
	}
}

func TestAssistantAttachmentsTransportAndOfficeExtraction(t *testing.T) {
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aUj8AAAAASUVORK5CYII=")
	image := chatAttachment("截图.png", "image/png", png)
	txt := chatAttachment("资料.md", "text/markdown", []byte("需要讨论的附件内容"))
	pdf := chatAttachment("资料.pdf", "application/pdf", []byte("%PDF-1.4 test document"))
	for _, provider := range []string{"openai", "ollama"} {
		t.Run(provider, func(t *testing.T) {
			s, w := fixture(t)
			testSecrets(s)
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				var body struct {
					Messages []map[string]any `json:"messages"`
				}
				if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
					t.Error(e)
				}
				message := body.Messages[len(body.Messages)-1]
				if provider == "openai" {
					parts, ok := message["content"].([]any)
					if !ok || len(parts) != 3 {
						t.Errorf("missing multimodal parts: %#v", message)
					} else {
						b, _ := json.Marshal(parts)
						for _, required := range []string{"image_url", "file_data", "需要讨论的附件内容"} {
							if !strings.Contains(string(b), required) {
								t.Error("missing", required)
							}
						}
					}
					fmt.Fprint(rw, `{"choices":[{"message":{"content":"读到了附件"}}]}`)
				} else {
					if images, ok := message["images"].([]any); !ok || len(images) != 1 || images[0] != image.Data {
						t.Error("missing Ollama image")
					}
					if !strings.Contains(message["content"].(string), "需要讨论的附件内容") {
						t.Error("text missing")
					}
					fmt.Fprint(rw, `{"message":{"content":"读到了附件"}}`)
				}
			}))
			defer server.Close()
			p := chatProvider(t, s, server.URL, provider)
			attachments := []AssistantAttachment{image, txt}
			if provider == "openai" {
				attachments = append(attachments, pdf)
			}
			result, e := s.ChatLLM(LLMChatInput{RequestID: uid(), WorkspaceID: w.ID, ProviderID: p.ID, Model: p.Model, Scope: "workspace", Messages: []LLMChatMessage{{Role: "user", Content: "分析附件", Attachments: attachments}}})
			if e != nil || result.Text != "读到了附件" {
				t.Fatal(result, e)
			}
		})
	}
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	f, _ := z.Create("word/document.xml")
	f.Write([]byte(`<w:document xmlns:w="urn:test"><w:p><w:r><w:t>正文来自Word</w:t></w:r></w:p></w:document>`))
	z.Close()
	a := chatAttachment("文档.docx", "application/octet-stream", archive.Bytes())
	value, err := assistantMessageContent("openai", "请阅读", []AssistantAttachment{a})
	if err != nil || !strings.Contains(value.(string), "正文来自Word") {
		t.Fatal(value, err)
	}
	if _, err = assistantMessageContent("ollama", "", []AssistantAttachment{pdf}); err == nil {
		t.Fatal("unsupported binary silently omitted")
	}
}

func TestAssistantAttachmentValidation(t *testing.T) {
	valid := chatAttachment("ok.txt", "text/plain", []byte("hello"))
	for _, mutate := range []func(*AssistantAttachment){func(a *AssistantAttachment) { a.Data = "invalid base64" }, func(a *AssistantAttachment) { a.Name = "../secret" }, func(a *AssistantAttachment) { a.ID = "../../other" }, func(a *AssistantAttachment) { a.MIME = "text/plain\nheader" }, func(a *AssistantAttachment) {
		a.Data = base64.StdEncoding.EncodeToString(make([]byte, assistantAttachmentLimit+1))
	}} {
		a := valid
		mutate(&a)
		if err := validateAssistantAttachments("user", []AssistantAttachment{a}); err == nil {
			t.Fatal("invalid attachment accepted")
		}
	}
	if err := validateAssistantAttachments("assistant", []AssistantAttachment{valid}); err == nil {
		t.Fatal("model attachment accepted")
	}
	if err := validateAssistantAttachments("user", []AssistantAttachment{valid, valid}); err == nil {
		t.Fatal("duplicate attachment id accepted")
	}
}
