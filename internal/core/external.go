package core

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

type secretStore interface {
	Get(string) (string, error)
	Set(string, string) error
	Delete(string) error
}
type systemSecrets struct{}

func (systemSecrets) Get(key string) (string, error) {
	v, e := keyring.Get("SmartNote", key)
	if errors.Is(e, keyring.ErrNotFound) {
		return "", nil
	}
	return v, e
}
func (systemSecrets) Set(key, value string) error { return keyring.Set("SmartNote", key, value) }
func (systemSecrets) Delete(key string) error {
	e := keyring.Delete("SmartNote", key)
	if errors.Is(e, keyring.ErrNotFound) {
		return nil
	}
	return e
}
func (s *Service) secretKey(scope string) string { return digest([]byte(s.root)) + ":" + scope }

// Credentials never cross an HTTP redirect. Remote services require TLS;
// plain HTTP is limited to a literal loopback host for local providers/tests.
func externalURL(raw string, localOnly bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("请输入不带账号、查询参数和片段的服务地址")
	}
	h := u.Hostname()
	ip := net.ParseIP(h)
	local := strings.EqualFold(h, "localhost") || ip != nil && ip.IsLoopback()
	if localOnly && !local {
		return nil, errors.New("Ollama 仅允许本机 localhost / 回环地址")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return nil, errors.New("远程服务必须使用 HTTPS；本机回环地址可使用 HTTP")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}
func externalClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func externalHTTPError(kind string, code int) error {
	return fmt.Errorf("%s失败（HTTP %d）；请检查服务地址、权限、模型名称或稍后重试", kind, code)
}
