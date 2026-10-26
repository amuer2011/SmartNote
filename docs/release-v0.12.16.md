# v0.12.16 导航与标签交互

选择模型后浮层保持打开，可继续切换或调整思考强度；点击外部或按 Escape 关闭。聊天输入区移除“默认”思考强度快捷按钮和“工作区/当前笔记”下拉框，思考滑杆保留在模型浮层底部。聊天查询范围固定为当前工作区，当前笔记和选中文本仍作为对话上下文。

单击导航、文件卡片或关联笔记默认替换当前标签；Ctrl 点击（Mac 为 Command 点击）在新标签打开。已打开的笔记会切换到已有标签，避免重复；空白标签可直接承接笔记。保留标签关闭、快捷键切换、保存冲突保护、上次笔记恢复和工作区隔离。

鼠标悬停文件/目录时显示柔和高亮，已经选中的项目继续保持高亮。悬停不改变真实焦点、选中笔记或删除目标。文件右键菜单以“打开”“在新标签页中打开”开头；目录保留新建笔记和文件夹。菜单移除名称标题及“刷新工作区”，其他重命名、剪切、移动、路径复制和删除操作继续保留。

笔记标签栏放在主笔记区域，不延伸至右侧辅助栏上方。标签过多时自动换行，超过屏幕高度的三成后可在标签区纵向滚动；切换标签或调整窗口/侧栏宽度时保证当前标签可见。

安装包：`build/bin/SmartNote-Setup-v0.12.16-navigation-tabs.exe`。免安装版：`build/bin/SmartNote-v0.12.16-navigation-tabs.exe`。

验证通过：`note-tabs-chat-attachments.cjs` 覆盖普通替换、Ctrl/Command 新开、去重、右键入口、悬停与持久高亮、多行标签边界、侧栏调整后的当前标签可见性、保存冲突、关闭及快捷键、聊天附件和工作区隔离。`model-picker.cjs` 验证选择后浮层保持打开、外部点击关闭、键盘、模型强度和主题。`sync-llm.cjs`、`workspace-switcher.cjs`、`view-navigation.cjs`、`note-session.cjs`、`navigation-search.cjs`、`replace-navigation.cjs` 回归通过；后者补齐默认折叠目录后的展开步骤。

前端类型检查、生产构建和全量 Go 测试通过，记录在 `.tmp/build-v01216.log`。独立数据目录下原生启动12秒响应正常；隔离安装、clean PATH 下内置 CPU 模型推理、卸载、用户数据保留和用户注册表/快捷方式不变验证通过。记录：`.tmp/native-v01216-test.log`、`.tmp/installer-v01216-test.log`；安装验证目录：`D:\CHANG\SmartNote-release-stage\installer-v091-803b644d`。界面和模型调用使用模拟数据验证，未向真实模型服务发送请求。

界面：[多行标签](preview-note-navigation-v0.12.16.png)、[模型浅色](preview-model-picker-light-v0.12.16.png)、[模型深色](preview-model-picker-dark-v0.12.16.png)。校验值见 [SHA-256](release-v0.12.16-sha256.json)。
