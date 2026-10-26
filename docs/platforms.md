# 平台与安装

所有发布包均包含中文语义模型及对应架构的 ONNX Runtime CPU 运行库。运行时不需要 Python、Ollama、显卡或模型下载。

| 平台 | 架构 | 发布格式 | 系统要求 |
| --- | --- | --- | --- |
| Windows | x64 | 安装器 EXE、免安装 EXE | Windows 10 / 11，WebView2 Runtime |
| macOS Intel | x64 | DMG、包含 `.app` 的 ZIP | macOS 13.3 或更高 |
| macOS Apple Silicon | ARM64 | DMG、包含 `.app` 的 ZIP | macOS 13.3 或更高，无需 Rosetta |
| Linux | x64、ARM64 | DEB、RPM、AppImage、TAR.GZ | glibc 2.35+、GTK 3、WebKitGTK 4.1；桌面会话 |

`x64` 对应 Intel/AMD 64 位处理器（DEB 名称为 `amd64`，RPM 为 `x86_64`）。`arm64` 对应 64 位 ARM（RPM 为 `aarch64`）。目前不提供 32 位、ARMv7、LoongArch 或 RISC-V 包。

## Windows

下载 `SmartNote-Setup-vX.Y.Z-windows-x64.exe` 安装，或直接运行同版本免安装 EXE。安装器按当前用户安装，卸载保留笔记与应用数据。

## macOS

按处理器选择 `macos-x64` 或 `macos-arm64` 的 DMG，将 `SmartNote.app` 拖入“应用程序”；ZIP 解压后同样可移动应用。

当前应用使用临时本机签名，尚未使用 Apple Developer ID 公证。首次从网络下载运行时，系统可能要求在“系统设置 → 隐私与安全性”允许打开。CI 会检查应用签名完整性、真实 CPU 模型推理与隔离桌面启动；不代表已完成所有 macOS 系统版本的人工验收。

## Linux

### Ubuntu、Debian 及兼容 DEB 的系统

Ubuntu 22.04 / 24.04、Debian 12 或更新的常见桌面环境可使用 DEB，让包管理器解析依赖：

```bash
sudo apt install ./SmartNote-vX.Y.Z-linux-x64.deb
smartnote
```

ARM64 使用对应的 `linux-arm64.deb`。统信、麒麟等兼容发行版，需要先确认系统提供 glibc 2.35+ 和 WebKitGTK 4.1；不能仅凭支持 DEB 格式判断兼容性。

### Fedora、openSUSE 及兼容 RPM 的系统

Fedora 使用 DNF 安装，openSUSE 使用 Zypper，选择与处理器匹配的 RPM：

```bash
sudo dnf install ./SmartNote-vX.Y.Z-linux-x64.rpm
# openSUSE
sudo zypper install ./SmartNote-vX.Y.Z-linux-x64.rpm
smartnote
```

RPM 记录 GTK / WebKit 的共享库依赖，由发行版软件源解析。较老的 CentOS、Rocky、AlmaLinux，以及部分国产发行版，可能缺少所需 glibc 或 WebKitGTK 版本；当前不承诺这些旧系统可以运行。

### Arch、Manjaro 与其他桌面发行版

可选 AppImage 或 TAR.GZ。Arch 系常见依赖为 `gtk3`、`webkit2gtk-4.1`，其他发行版按对应名称安装。

```bash
chmod +x SmartNote-vX.Y.Z-linux-x64.AppImage
./SmartNote-vX.Y.Z-linux-x64.AppImage

# 系统无法使用 FUSE 时，运行库支持免挂载解包运行
./SmartNote-vX.Y.Z-linux-x64.AppImage --appimage-extract-and-run

# 也可解压便携包后运行
tar -xzf SmartNote-vX.Y.Z-linux-x64.tar.gz
./SmartNote-vX.Y.Z-linux-x64/SmartNote
```

AppImage 内含应用、离线模型与推理运行库；GTK 3、WebKitGTK 4.1 和显示服务由系统提供。AppImage 不会让较老的 glibc / WebKit 系统自动获得兼容性。Alpine/musl 系统不在当前支持范围。

## 构建与验证

Windows 使用 `build.ps1`。macOS/Linux 在对应原生架构安装 Go、Node.js 22、Python 3.10+ 后执行 `python3 build/unix.py`。

Linux 构建依赖（Ubuntu 22.04）：

```bash
sudo apt update
sudo apt install build-essential ca-certificates curl git pkg-config libgtk-3-dev \
  libwebkit2gtk-4.1-dev python3 rpm cpio squashfs-tools desktop-file-utils xvfb xauth dbus-x11
python3 build/unix.py
```

无桌面的 CI 环境可用 `xvfb-run -a dbus-run-session -- python3 build/unix.py`。只有以 root 运行的隔离 CI 容器需要额外关闭 WebKit 沙箱，普通用户不需要这样设置。

CI 在原生 Intel / ARM Mac、原生 x64 / ARM64 Linux 以及 Windows 上构建。Linux 在 Ubuntu 22.04 容器中构建以保持 glibc 基线；检查前端类型、Go 测试、CPU 模型推理、DEB/RPM 解包、AppImage 实际执行和隔离桌面启动。包格式覆盖多种发行版，不代表已在每一种发行版及每一代系统上做过完整交互验收。

Linux AI / WebDAV 密钥存储使用桌面 Secret Service，通常由 GNOME Keyring 或兼容服务提供；macOS 使用系统钥匙串。
