package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func uid() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".smartnote-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, b)
}
func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// safePath rejects absolute paths, traversal, platform-specific aliases and symlinks.
func safePath(root, rel string) (string, error) {
	rel = filepath.FromSlash(rel)
	if rel == "" || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" || strings.ContainsAny(rel, ":\x00") {
		return "", errors.New("非法相对路径")
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("路径不能越出工作区")
	}
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		upper := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if strings.TrimRight(part, " .") != part || upper == "CON" || upper == "PRN" || upper == "AUX" || upper == "NUL" || (len(upper) == 4 && (strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT")) && upper[3] >= '0' && upper[3] <= '9') {
			return "", errors.New("不支持的文件名")
		}
	}
	target := filepath.Join(root, clean)
	for p := target; p != root; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("不允许访问符号链接")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return target, nil
}

func visibleNotePath(rel string) error {
	for _, part := range strings.Split(filepath.ToSlash(filepath.Clean(rel)), "/") {
		if strings.HasPrefix(part, ".") {
			return errors.New("笔记不能保存到隐藏文件或隐藏目录；这些路径不会被工作区扫描")
		}
	}
	return nil
}
