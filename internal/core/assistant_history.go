package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type AssistantMemory struct {
	Revision string `json:"revision"`
	Text     string `json:"text"`
}
type AssistantEntry struct {
	Attachments []AssistantAttachment `json:"attachments,omitempty"`
	Role        string                `json:"role"`
	Content     string                `json:"content"`
	Sources     []LLMSource           `json:"sources"`
	Service     string                `json:"service,omitempty"`
	Notice      bool                  `json:"notice,omitempty"`
}
type AssistantConversation struct {
	ID       string           `json:"id"`
	Revision string           `json:"revision"`
	Title    string           `json:"title"`
	Updated  string           `json:"updated"`
	Entries  []AssistantEntry `json:"entries"`
}
type AssistantConversationInfo struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Updated  string `json:"updated"`
	Messages int    `json:"messages"`
}

var assistantConversationID = regexp.MustCompile(`^[a-f0-9]{32}$`)

func (s *Service) assistantMemory(w Workspace) (AssistantMemory, error) {
	var memory AssistantMemory
	err := readJSON(filepath.Join(s.data(w), "assistant-memory.json"), &memory)
	if err == nil && len([]rune(memory.Text)) > 8000 {
		err = errors.New("长期背景最多8000字，请在管理窗口调整")
	}
	return memory, err
}
func (s *Service) GetAssistantMemory(workspaceID string) (AssistantMemory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return AssistantMemory{}, err
	}
	return s.assistantMemory(w)
}
func (s *Service) SaveAssistantMemory(workspaceID string, in AssistantMemory) (AssistantMemory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return AssistantMemory{}, err
	}
	if len([]rune(in.Text)) > 8000 {
		return AssistantMemory{}, errors.New("长期背景最多8000字")
	}
	old, err := s.assistantMemory(w)
	if err != nil {
		return AssistantMemory{}, err
	}
	if old.Revision != in.Revision {
		return AssistantMemory{}, errors.New("长期背景已变化，请重新打开后保存")
	}
	in.Text = strings.TrimSpace(in.Text)
	in.Revision = uid()
	err = writeJSON(filepath.Join(s.data(w), "assistant-memory.json"), in)
	return in, err
}
func (s *Service) assistantConversationPath(w Workspace, id string) (string, error) {
	if !assistantConversationID.MatchString(id) {
		return "", errors.New("无效对话编号")
	}
	return safePath(s.data(w), "assistant/conversations/"+id+".json")
}
func loadAssistantConversation(path string) (AssistantConversation, error) {
	var out AssistantConversation
	info, err := os.Stat(path)
	if err != nil {
		return out, err
	}
	if info.Size() > assistantConversationLimit {
		return out, errors.New("对话记录过大")
	}
	err = readJSON(path, &out)
	if err == nil {
		err = validateAssistantConversation(out)
	}
	return out, err
}
func (s *Service) ListAssistantConversations(workspaceID string) ([]AssistantConversationInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	out := []AssistantConversationInfo{}
	dir, err := safePath(s.data(w), "assistant/conversations")
	if err != nil {
		return nil, err
	}
	files, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		id := strings.TrimSuffix(file.Name(), ".json")
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") || !assistantConversationID.MatchString(id) {
			continue
		}
		path, e := s.assistantConversationPath(w, id)
		if e != nil {
			return nil, e
		}
		c, e := loadAssistantConversation(path)
		if e != nil {
			return nil, errors.New("对话历史读取失败，请保留本地数据后重试")
		}
		out = append(out, AssistantConversationInfo{ID: id, Title: c.Title, Updated: c.Updated, Messages: len(c.Entries)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated > out[j].Updated })
	return out, nil
}
func (s *Service) GetAssistantConversation(workspaceID, id string) (AssistantConversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return AssistantConversation{}, err
	}
	path, err := s.assistantConversationPath(w, id)
	if err != nil {
		return AssistantConversation{}, err
	}
	return loadAssistantConversation(path)
}
func (s *Service) SaveAssistantConversation(workspaceID string, in AssistantConversation) (AssistantConversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return in, err
	}
	path, err := s.assistantConversationPath(w, in.ID)
	if err != nil {
		return in, err
	}
	if err = validateAssistantConversation(in); err != nil {
		return in, err
	}
	old, err := loadAssistantConversation(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return in, err
	}
	if in.Revision != old.Revision {
		return in, errors.New("此对话已在其他窗口更新，请保留当前文字并重新打开历史记录")
	}
	if in.Title == "" {
		for _, entry := range in.Entries {
			if entry.Role == "user" {
				in.Title = assistantTruncate(strings.TrimSpace(entry.Content), 40)
				break
			}
		}
	}
	if len([]rune(in.Title)) > 80 {
		return in, errors.New("对话标题最多80字")
	}
	in.Revision = uid()
	in.Updated = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return in, err
	}
	if len(b) > assistantConversationLimit {
		return in, errors.New("此对话记录已满，请新建对话继续")
	}
	err = atomicWrite(path, b)
	return in, err
}
func (s *Service) DeleteAssistantConversation(workspaceID, id, revision string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return err
	}
	path, err := s.assistantConversationPath(w, id)
	if err != nil {
		return err
	}
	old, err := loadAssistantConversation(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if old.Revision != revision {
		return errors.New("对话已变化，请重新打开历史记录")
	}
	return os.Remove(path)
}

func validateAssistantConversation(in AssistantConversation) error {
	attachmentTotal := 0
	if !assistantConversationID.MatchString(in.ID) || len([]rune(in.Title)) > 80 {
		return errors.New("无效对话编号或标题过长")
	}
	if len(in.Entries) == 0 || len(in.Entries) > 400 {
		return errors.New("单个对话最多保存400条消息，请新建对话继续")
	}
	for _, entry := range in.Entries {
		for _, a := range entry.Attachments {
			attachmentTotal += len(a.Data)
		}
		if attachmentTotal > 32<<20 {
			return errors.New("当前对话附件已满，请新建对话")
		}
		if err := validateAssistantAttachments(entry.Role, entry.Attachments); err != nil {
			return err
		}
		if entry.Role != "user" && entry.Role != "assistant" || len([]rune(entry.Content)) > 100000 || len(entry.Sources) > 20 || len(entry.Service) > 500 {
			return errors.New("无效对话记录或消息过长")
		}
	}
	return nil
}
func assistantConversationFiles(root string) ([]string, error) {
	dir, err := safePath(root, "assistant/conversations")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || !assistantConversationID.MatchString(strings.TrimSuffix(entry.Name(), ".json")) {
			continue
		}
		rel := "assistant/conversations/" + entry.Name()
		if _, err = safePath(root, rel); err != nil {
			return nil, err
		}
		files = append(files, rel)
	}
	return files, nil
}
func validateAssistantMetadata(root string) error {
	var memory AssistantMemory
	if err := readJSON(filepath.Join(root, "assistant-memory.json"), &memory); err != nil {
		return err
	}
	if len([]rune(memory.Text)) > 8000 {
		return errors.New("长期背景最多8000字")
	}
	files, err := assistantConversationFiles(root)
	if err != nil {
		return err
	}
	for _, file := range files {
		c, err := loadAssistantConversation(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return err
		}
		if c.ID != strings.TrimSuffix(filepath.Base(file), ".json") {
			return errors.New("对话编号与文件名不一致")
		}
	}
	return nil
}
