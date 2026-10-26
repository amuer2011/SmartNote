# Actions 与版本发布

工作流：[Build and release](../.github/workflows/release.yml)。

推送 `main`、向 `main` 提交 PR 或在 Actions 页面手动运行时，同时构建 Windows x64、macOS Intel、macOS ARM64、Linux x64 和 Linux ARM64。推送 `vX.Y.Z` 标签时，所有平台通过后自动发布 Release，标题仅为版本号，例如 `v1.0.1`。

普通作业仅有仓库读取权限，发布作业单独使用 `contents: write`；使用自动提供的 `GITHUB_TOKEN`，无需额外配置密钥。官方 Actions 固定到具体提交。

## 构建与验收

- Windows：Windows 2022、MinGW-w64、`build.ps1`，包含隔离安装、离线推理、卸载及数据保留检查。NSIS 使用固定校验值的本机工具或仓库自带官方归档。
- macOS：原生 `macos-15-intel` 与 `macos-15` runner，分别构建 x64 和 ARM64 的 APP、ZIP、DMG，校验临时签名、真实推理和隔离桌面启动。
- Linux：原生 x64 / ARM64 runner，在 Ubuntu 22.04 容器中构建，保留 glibc 2.35 基线。分别生成 DEB、RPM、AppImage、TAR.GZ，检查原生推理、DEB/RPM 解包、AppImage 实际执行和 Xvfb 桌面启动。

所有平台均执行前端类型检查、生产构建及 Go 测试。平台运行库从 [固定的 ONNX 来源](../build/native-runtimes.json)下载并校验，编入程序；终端用户无需下载。Linux AppImage 使用[固定版本运行库](../build/appimage-runtimes.json)与 squashfs-tools 打包。来源及许可保留在安装包中。

系统要求、签名说明与兼容边界见[平台说明](platforms.md)。包格式覆盖常见发行版，当前不会宣称在所有发行版、所有系统版本上完成全面交互验收。

## 发布下一版本

1. 修改 `frontend/package.json` 和 `frontend/package-lock.json` 中根包版本。
2. 新增 `docs/release-vX.Y.Z.md`，更新 README 版本和下载说明。
3. 提交并推送 `main`，确认所有平台 CI 通过。
4. 创建并推送与包版本完全一致的标签：

```powershell
git tag -a v1.0.2 -m "v1.0.2"
git push origin v1.0.2
```

构建时检查标签、包版本和说明文件。界面、Windows 文件属性、安装器、macOS Info.plist、DEB/RPM 版本均使用包版本。

## 发布产物

一个版本包含 14 个平台文件：Windows 两个 EXE；macOS 两种架构各一个 ZIP 和 DMG；Linux 两种架构各一个 DEB、RPM、AppImage 和 TAR.GZ。

每个平台输出独立校验清单。发布作业聚合所有文件，检查总数及 SHA-256，生成统一 `SHA256SUMS.txt`。先创建草稿、上传全部文件，再公开 Release。失败留下草稿时可以重跑；已公开版本不会被工作流覆盖。现有正式标签保留，源码变更通过新补丁版本发布。

平台构建产物在 Actions 中保留 14 天，正式 Release 附件按 GitHub Release 规则保存。
