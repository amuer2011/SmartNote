# v0.12.6 提示块与 AI 聊天窗口

安装包：`build/bin/SmartNote-Setup-v0.12.6-admonition-chat.exe`。免安装版：`build/bin/SmartNote-v0.12.6-admonition-chat.exe`。

AI 聊天右上角增加“浮动 AI 聊天”按钮。浮动后可拖动标题栏移动位置，拖动四边或四角改变窗口大小；位置和尺寸在本机自动记忆。再次点击停靠按钮，或点击浮动窗口的关闭/最小化按钮，返回原侧栏。切换过程中沿用同一个助手实例，保留会话、草稿、模型选择和正在生成的回复。切换侧栏页签或隐藏侧栏时沿用原有显示与请求处理规则。

聊天输入框上沿增加高度拖动条，向上拖动可扩大输入区，也可聚焦拖动条后使用上下方向键调整。高度单独记忆，窗口缩小时自动适应可用空间。侧栏模式继续通过原有分隔条调整宽度。

笔记预览、分屏预览及 AI 回复内置 Admonition 常用语法兼容，无需安装 Obsidian。兼容 `ad-*` 代码围栏及 Obsidian Callout，支持自定义标题、折叠、嵌套、内部 Markdown、列表、表格、链接和本地附件。编辑工具栏增加“插入提示块”，可包裹选中的文本。

```markdown
> [!warning]- 可折叠的提醒
> 这里支持 **加粗**、列表及链接。
```

`[!note]` 表示普通提示；`[!tip]`、`[!info]`、`[!success]`、`[!question]`、`[!warning]`、`[!danger]`、`[!bug]`、`[!example]`、`[!quote]` 等提供不同样式。类型后加 `-` 默认折叠，加 `+` 默认展开且可折叠。支持 summary、tldr、hint、important、check、done、help、faq、caution、attention、fail、missing、error、cite 等常见别名。

````markdown
```ad-tip
title: 自定义标题
collapse: open
## 内容
- 一条技巧
- 另一条技巧
```
````

围栏中的 `collapse: open` / `closed` 控制默认展开/折叠。不配置则显示普通提示块。保持标准 Markdown 源文，普通引用和代码块保持原有渲染。此功能是语法兼容，不提供完整 Obsidian 插件运行环境、自定义脚本或插件专用设置。继续使用 HTML 转义及 DOMPurify 过滤。

验证：

- `frontend/tests/admonitions.cjs`：围栏、Callout、别名、嵌套、折叠、内部 Markdown、源行映射、普通代码保留及安全文本渲染。
- `frontend/tests/admonition-chat.cjs`：真实鼠标折叠/展开、隐藏内容查找自动展开、本地链接、前置元数据后的源行；聊天四边与角落缩放、标题栏移动、输入区高度调整、草稿与在途请求保留、隐藏/恢复、刷新后尺寸恢复和小窗口适配。
- `sync-llm.cjs`：聊天历史、模型选择、思考选项、侧栏页签、修改审阅、分屏与预览大纲等既有流程回归通过。
- `view-navigation.cjs`：预览大纲与查找、分屏查找目标、拓扑覆盖与恢复、视口保持及独立快捷键设置通过；测试按现有界面先展开目录，并从“关联”页签打开拓扑。
- `editor-document.cjs`、`editor-attachments.cjs`：编辑器源文与行号、工具栏选区、附件预览、图片窗口及图片上传开关等回归通过。

浏览器验证使用隔离模拟服务，未操作用户笔记或真实模型接口。前端类型检查、生产构建及全量 Go 测试通过。生产程序在独立数据目录启动12秒并正常响应；隔离安装/卸载、clean PATH 内置 CPU 推理（512维，约533毫秒）、用户文件保留、真实用户注册表和快捷方式不变验证通过。证据：`.tmp/build-v0126.log`、`.tmp/native-v0126-test.log`、`.tmp/installer-v0126-test.log`，隔离安装目录为 `D:/CHANG/SmartNote-release-stage/installer-v091-333dbd70`。校验值见 [SHA-256](release-v0.12.6-sha256.json)。
