# Actions 与版本发布

工作流：[Windows build and release](../.github/workflows/windows.yml)。

## 自动构建

- 推送 `main`、向 `main` 提交 PR，或在 Actions 页面手动运行：构建 Windows x64 程序与安装包，上传保留 14 天的构建产物。
- 推送 `vX.Y.Z` 标签：执行同样的构建和检查，通过后自动创建并发布 GitHub Release。
- 普通构建只有仓库读取权限；发布作业单独获得 `contents: write`，使用自动提供的 `GITHUB_TOKEN`，无需额外配置密钥。

构建环境固定为 `windows-2022`，使用 `go.mod` 指定的 Go 版本、Node.js 22、MinGW-w64 GCC 和 npm 锁定依赖。内置模型与运行库按清单校验，NSIS 安装器编译工具使用固定版本与 SHA-256，优先复用校验一致的已安装版本，下载时依次尝试官方镜像且仅接受校验一致的归档。工作流使用官方 Actions，并固定到具体提交。

检查包括前端 TypeScript 与生产构建、全量 Go 测试、隔离安装后的离线 CPU 模型自检、卸载和用户数据保留。发布前再次检查产物 SHA-256。

## 发布下一版本

1. 修改 `frontend/package.json` 的版本，并同步 `frontend/package-lock.json` 中根包的版本。
2. 新增 `docs/release-vX.Y.Z.md`，更新 README 的版本和下载说明。
3. 提交并推送 `main`，确认 Actions 构建通过。
4. 创建并推送与版本完全一致的标签：

```powershell
git tag -a v1.0.1 -m "SmartNote v1.0.1"
git push origin v1.0.1
```

发布标签必须与 `frontend/package.json` 一致。构建脚本以该版本生成界面、Windows 文件属性、应用清单与安装器版本；源模板中的版本在构建时会被覆盖。

## 产物

- `SmartNote-vX.Y.Z-windows-x64.exe`：免安装版。
- `SmartNote-Setup-vX.Y.Z-windows-x64.exe`：安装包。
- `SHA256SUMS.txt`：SHA-256 校验清单。

发布时先创建草稿，上传并校验完整产物后再公开，避免显示没有安装包的版本。若失败时留下草稿，可修复原因后重跑发布作业；已公开版本不会被工作流覆盖，应使用新版本发布。

本地同样可构建和验收：

```powershell
.\build.ps1 -Installer -Output build/bin/SmartNote-v1.0.0-windows-x64.exe
.\build\Test-Installer.ps1 `
  -Installer build/bin/SmartNote-Setup-v1.0.0-windows-x64.exe `
  -Binary build/bin/SmartNote-v1.0.0-windows-x64.exe
```
