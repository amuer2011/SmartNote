# v0.12.7 提示块输入补全

安装包：`build/bin/SmartNote-Setup-v0.12.7-admonition-completion.exe`。免安装版：`build/bin/SmartNote-v0.12.7-admonition-completion.exe`。

编辑器输入以下内容时，自动在光标附近弹出紧凑的提示列表：

````markdown
```ad-
````

列表按参考图排列：note、seealso、abstract、summary、tldr、info、todo、tip、hint 等，包含28种常见类型与别名，类型名称以颜色区分，右侧显示对应图标。继续输入名称前缀即可筛选。上下方向键选择，Enter 或 Tab 确认，Esc 收起；也支持鼠标点击。列表可以滚动，在窗口边缘自动调整显示位置，并跟随明暗主题、界面字体和字号。

Obsidian Callout 也提供相同补全：输入 `> [!` 后选择类型，自动补上缺失的右方括号。支持引文中的围栏及波浪线围栏。补全只替换类型范围，保留已有标题、折叠标记、正文、闭合围栏和隐藏的笔记元数据，支持撤销。

类型列表与预览渲染器共享数据，seealso 按 note 样式渲染。普通正文、行内代码及其他代码块的内容不会触发 Admonition 补全。补全打开时，Esc 优先关闭提示列表，保持查找栏和分屏视图。

验证：

- `admonition-completion.cjs`：自动触发、类型顺序、图标、颜色和明暗主题、前缀筛选、上下/Enter/Tab/Esc/鼠标操作、撤销、原有标题和正文、Callout 方括号、引文和波浪线围栏、代码排除、元数据保留及小窗口显示通过。
- `view-navigation.cjs`：在主界面验证补全确认及 Esc 操作保持查找栏与分屏；预览定位、查找目标、图谱覆盖/恢复及快捷键设置回归通过。
- `admonitions.cjs`、`admonition-chat.cjs`：提示块渲染、折叠、嵌套、源行、查找和链接，以及聊天浮窗缩放、停靠、草稿和请求保留、尺寸记忆回归通过。
- `editor-attachments.cjs`：原文元数据、工具栏选区、附件与图片操作等回归通过。

浏览器验证使用隔离数据，未操作用户笔记或真实模型服务。前端类型检查、生产构建及全量 Go 测试通过。生产程序在独立数据目录启动12秒并正常响应；隔离安装、clean PATH 内置 CPU 推理、卸载和用户数据保留检查通过，用户注册表和快捷方式保持不变。日志见 `.tmp/build-v0127.log`、`.tmp/native-v0127-test.log`、`.tmp/installer-v0127-test.log`；校验值见 [SHA-256](release-v0.12.7-sha256.json)。
