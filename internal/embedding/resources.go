package embedding

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Resources are embedded in both the portable program and the installed program.
//
//go:embed assets/config.json assets/manifest.json assets/model.onnx assets/tokenizer.json assets/tokenizer_config.json assets/vocab.txt assets/*LICENSE assets/MODEL-README.md assets/ORT-ThirdPartyNotices.txt assets/VC-LICENSE.html
var resources embed.FS

var platformResources fs.FS
var additionalResources fs.FS

type nativeManifest struct {
	Platform string              `json:"platform"`
	Runtime  string              `json:"runtime"`
	Files    map[string]Resource `json:"files"`
}

const ModelName = "BAAI/bge-small-zh-v1.5"
const Dimensions = 512

type Resource struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type Manifest struct {
	Model    string              `json:"model"`
	Revision string              `json:"revision"`
	Runtime  string              `json:"runtime"`
	Files    map[string]Resource `json:"files"`
}

func resourceManifest() (Manifest, []byte, error) {
	b, e := resources.ReadFile("assets/manifest.json")
	if e != nil {
		return Manifest{}, nil, e
	}
	var m Manifest
	e = json.Unmarshal(b, &m)
	if e != nil || platformResources == nil {
		return m, b, e
	}
	raw, err := fs.ReadFile(platformResources, "runtime.json")
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("本机运行库未准备，请先运行 build/prepare-runtime.py：%w", err)
	}
	var native nativeManifest
	if err = json.Unmarshal(raw, &native); err != nil {
		return Manifest{}, nil, err
	}
	if native.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return Manifest{}, nil, fmt.Errorf("运行库平台不匹配：%s", native.Platform)
	}
	for name := range m.Files {
		if strings.HasSuffix(name, ".dll") {
			delete(m.Files, name)
		}
	}
	if len(native.Files) == 0 {
		return Manifest{}, nil, fmt.Errorf("本机运行库清单为空")
	}
	for name, entry := range native.Files {
		if filepath.Base(name) != name || strings.ContainsAny(name, `/\`) || entry.Size <= 0 {
			return Manifest{}, nil, fmt.Errorf("无效的运行库资源：%s", name)
		}
		if digest, err := hex.DecodeString(entry.SHA256); err != nil || len(digest) != sha256.Size {
			return Manifest{}, nil, fmt.Errorf("无效的运行库校验值：%s", name)
		}
		m.Files[name] = entry
	}
	m.Runtime = native.Runtime
	b, e = json.Marshal(m)
	return m, b, e
}

func resourceBytes(name string) ([]byte, error) {
	if platformResources != nil {
		b, err := fs.ReadFile(platformResources, name)
		if err == nil {
			return b, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if additionalResources != nil {
		b, err := fs.ReadFile(additionalResources, name)
		if err == nil {
			return b, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return resources.ReadFile("assets/" + name)
}
func fileHash(path string) (string, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
func unpack(root string) (string, error) {
	m, raw, e := resourceManifest()
	if e != nil {
		return "", e
	}
	version := sha256.Sum256(raw)
	dir := filepath.Join(root, fmt.Sprintf("bge-small-zh-v1.5-%x", version[:6]))
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	for name, item := range m.Files {
		if filepath.Base(name) != name {
			return "", fmt.Errorf("invalid resource name: %s", name)
		}
		dest := filepath.Join(dir, name)
		if info, e := os.Lstat(dest); e == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("模型资源不允许符号链接：%s", name)
		}
		if hash, e := fileHash(dest); e == nil && hash == item.SHA256 {
			continue
		}
		b, e := resourceBytes(name)
		if e != nil {
			return "", e
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != item.SHA256 || int64(len(b)) != item.Size {
			return "", fmt.Errorf("内置模型资源校验失败：%s", name)
		}
		f, e := os.CreateTemp(dir, ".resource-")
		if e != nil {
			return "", e
		}
		tmp := f.Name()
		_, e = f.Write(b)
		if e == nil {
			e = f.Sync()
		}
		closeErr := f.Close()
		if e == nil {
			e = closeErr
		}
		if e == nil {
			e = os.Rename(tmp, dest)
		}
		os.Remove(tmp)
		if e != nil {
			return "", e
		}
	}
	return dir, nil
}

// Fingerprint identifies the bundled model, tokenizer and preprocessing contract.
func Fingerprint() string {
	raw, _ := resources.ReadFile("assets/manifest.json")
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%s:%x:passages-v1", ModelName, sum)
}
