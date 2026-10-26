# v0.9.0 离线模型交付验证

验证日期：2026-10-15，Windows x64。

| 产物 | 字节数 | SHA-256 |
| --- | ---: | --- |
| SmartNote-Setup-v0.9.0.exe | 24850815 | `3a2afa3714aada87b6baabd5f26b4ce3260fcd565693c15b6ce2c1bdf80fafdc` |
| SmartNote-v0.9.0.exe | 55450624 | `c23ad2cca790ab465fdc2c0cad585624cbd8c9c32b8d9da55c022b72cc883929` |

验证结果：

- `build.ps1 -SkipInstall -Installer -Output build/bin/SmartNote-v0.9.0.exe` 通过资源哈希校验、前端类型检查/构建、全部 Go 测试及安装器编译。
- 官方 Tokenizer 对照通过，覆盖中文、大小写、重音、特殊符号、清理和截断。实际模型采用原始的区分大小写配置。
- 真实模型的数据库/旅行/会议/健康四类样本分别匹配对应默认目录；低匹配、配置变化、旧版配置迁移、备份、移动和撤销也有测试。
- `node frontend/tests/ui-smoke.cjs` 完整回归通过，零浏览器错误，包括内置状态、分类 CRUD、自检、Inbox 自动归档/撤销及原有编辑器交互。
- 桌面程序使用独立测试配置启动，WebView2 创建成功，非管理员 Medium 完整性；检查后只结束本次测试进程。
- 最终安装器使用 `build/Test-Installer.ps1` 在独立目录安装，安装的程序哈希与免安装版一致。PATH 仅包含 Windows 系统目录时，内置模型自检通过：512 维，相关相似度 0.792、无关相似度 0.236，首次准备与三条推理约 541 ms。该时间仅代表本机小样本。
- 安装器测试没有修改用户现有注册记录或快捷方式；卸载移除了程序并保留测试用户数据和额外用户文件。最终证据：`D:\CHANG\SmartNote-release-stage\installer-v090-c4e6dee6`。

内置模型和运行库无需联网下载，不依赖 Ollama、Python、Go、GCC 或显卡。推理库遥测已关闭。应用仍要求系统具备 WebView2 Runtime。未进行真实用户笔记的分类搬动；样本结果不代表对所有笔记的准确率保证。
