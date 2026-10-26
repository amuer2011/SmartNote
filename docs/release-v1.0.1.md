# v1.0.1

增加 macOS 与常见 Linux 架构的原生构建、离线模型及安装包。Release 标题统一使用版本号。

| 平台 | 架构 | 下载格式 |
| --- | --- | --- |
| Windows | x64 | 安装包 EXE、免安装 EXE |
| macOS Intel | x64 | DMG、APP ZIP |
| macOS Apple Silicon | ARM64 | DMG、APP ZIP |
| Linux | x64 | DEB、RPM、AppImage、TAR.GZ |
| Linux | ARM64 | DEB、RPM、AppImage、TAR.GZ |

所有平台保留离线语义检索与自动分类，安装包已内置对应架构的 ONNX CPU 运行库。新增运行库平台与完整性校验，防止加载错误架构或损坏资源；Windows 保留原有安全 DLL 加载方式。

GitHub Actions 同时构建 5 个系统/架构组合，检查前端类型、Go 测试、真实模型推理及隔离桌面启动；macOS 验证应用签名，Linux 验证 DEB/RPM 解包与 AppImage 实际执行。全部成功后发布 14 个平台文件和统一 `SHA256SUMS.txt`。

Windows 需要 WebView2。macOS 需要 13.3+，当前为临时签名，尚无 Apple Developer ID 公证。Linux 需要 glibc 2.35+、GTK 3、WebKitGTK 4.1；DEB 面向 Ubuntu/Debian 兼容系统，RPM 面向具备对应依赖的 Fedora/openSUSE 等系统，其他发行版可使用 AppImage 或 TAR.GZ。

详细系统依赖、安装方法、签名与验证范围见 [平台说明](https://github.com/amuer2011/SmartNote/blob/v1.0.1/docs/platforms.md)。现有 v1.0.0 标签保留，本版新增平台支持以补丁版本交付。
