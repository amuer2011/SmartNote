package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type davClient struct {
	base               *url.URL
	username, password string
	http               *http.Client
}

func newDAV(c SyncConfig, secret string) (*davClient, error) {
	u, e := externalURL(c.Endpoint, false)
	if e != nil {
		return nil, e
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + c.RemoteID
	return &davClient{u, c.Username, secret, externalClient(60 * time.Second)}, nil
}
func (d *davClient) request(ctx context.Context, method, name string, data []byte, headers map[string]string) (*http.Response, error) {
	u := *d.base
	u.Path += "/" + name
	req, e := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(data))
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "SmartNote/0.10 WebDAV")
	if d.username != "" || d.password != "" {
		req.SetBasicAuth(d.username, d.password)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, e := d.http.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return nil, errors.New("同步已取消或超时")
		}
		return nil, errors.New("无法连接 WebDAV，请检查网络和服务地址")
	}
	return res, nil
}
func (d *davClient) mkdir(ctx context.Context) error {
	// The configured server collection must exist; only the workspace collection
	// and its immutable object collection are created by SmartNote.
	for _, name := range []string{"", "objects"} {
		r, e := d.request(ctx, "MKCOL", name, nil, nil)
		if e != nil {
			return e
		}
		r.Body.Close()
		if r.StatusCode != 201 && r.StatusCode != 405 && r.StatusCode != 200 && r.StatusCode != 204 {
			return externalHTTPError("创建同步目录", r.StatusCode)
		}
	}
	return nil
}
func (d *davClient) manifest(ctx context.Context) (syncManifest, string, error) {
	m := syncManifest{Version: 1, WorkspaceID: strings.TrimPrefix(d.base.Path[strings.LastIndex(d.base.Path, "/"):], "/"), Files: map[string]syncFile{}}
	r, e := d.request(ctx, "GET", "workspace.json", nil, nil)
	if e != nil {
		return m, "", e
	}
	defer r.Body.Close()
	if r.StatusCode == 404 {
		return m, "", nil
	}
	if r.StatusCode != 200 {
		return m, "", externalHTTPError("读取同步清单", r.StatusCode)
	}
	etag := r.Header.Get("ETag")
	if etag == "" || strings.HasPrefix(etag, "W/") {
		return m, "", errors.New("WebDAV 未提供强 ETag，无法安全同步；请使用支持条件请求的服务")
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, 16<<20+1))
	if e != nil || len(b) > 16<<20 {
		return m, "", errors.New("同步清单过大或无法读取")
	}
	if json.Unmarshal(b, &m) != nil {
		return m, "", errors.New("远端同步清单格式无效")
	}
	return m, etag, nil
}
func (d *davClient) putObject(ctx context.Context, hash string, b []byte) error {
	r, e := d.request(ctx, "PUT", "objects/"+hash, b, map[string]string{"If-None-Match": "*", "Content-Type": "application/octet-stream"})
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode == 412 { // Content addressed names are verified before trusting an existing object.
		stored, e := d.getObject(ctx, hash)
		if e != nil {
			return e
		}
		if !bytes.Equal(stored, b) {
			return errors.New("远端对象与内容哈希不符")
		}
		return nil
	}
	if r.StatusCode != 200 && r.StatusCode != 201 && r.StatusCode != 204 {
		return externalHTTPError("上传文件", r.StatusCode)
	}
	return nil
}
func (d *davClient) getObject(ctx context.Context, hash string) ([]byte, error) {
	if !validDigest(hash) {
		return nil, errors.New("无效同步哈希")
	}
	r, e := d.request(ctx, "GET", "objects/"+hash, nil, nil)
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, externalHTTPError("下载文件", r.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, maxAttachment+1))
	if e != nil || len(b) > maxAttachment {
		return nil, errors.New("下载文件超过 100 MB 或读取失败")
	}
	if digest(b) != hash {
		return nil, errors.New("下载文件校验失败，原文件未覆盖")
	}
	return b, nil
}
func (d *davClient) commit(ctx context.Context, m syncManifest, etag string) error {
	b, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return e
	}
	h := map[string]string{"Content-Type": "application/json"}
	if etag == "" {
		h["If-None-Match"] = "*"
	} else {
		h["If-Match"] = etag
	}
	r, e := d.request(ctx, "PUT", "workspace.json", b, h)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode == 412 {
		return errors.New("另一台设备正在同步，已保留本地内容，请重试")
	}
	if r.StatusCode != 200 && r.StatusCode != 201 && r.StatusCode != 204 {
		return externalHTTPError("提交同步清单", r.StatusCode)
	}
	return nil
}
