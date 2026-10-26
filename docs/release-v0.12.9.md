# v0.12.9 工作区切换与数据隔离

安装包：`build/bin/SmartNote-Setup-v0.12.9-workspace-switcher.exe`。免安装版：`build/bin/SmartNote-v0.12.9-workspace-switcher.exe`。

顶部工作区按钮直接显示已打开的工作区，不再打开独立页面。默认按注册顺序展示前5个；超过5个时，底部向下箭头展开全部。菜单根据屏幕可用高度显示，列表超出后滚动，底部“新建工作区”始终可见。新建入口打开本地目录选择器，可选择或创建目录。当前工作区的重命名、移除注册入口保留在菜单右上角，移除注册保留本地文件。

导航只加载当前工作区目录。切换后，笔记、搜索、知识拓扑、待办与完成归档、附件、书签、回收站、任务提醒及 AI 对话和长期背景均使用当前工作区的数据。供应商配置、主题字体等通用偏好，以及按年份缓存的农历和节假日继续共享。

切换前保存当前笔记。存在保存冲突、打开失败或取消新建时，保留原工作区和笔记草稿。新工作区内容读取失败时回退到原工作区。切换期间暂停界面操作，工作区与其内容一起更新，旧快照或 AI 回复迟到时不会覆盖新工作区。待办筛选、回收预览、附件预览、搜索和历史导航等旧状态会重置；已最小化窗口仅在所属工作区显示恢复入口。

验证：

- `workspace-switcher.cjs`：14个工作区的前5项、展开滚动、固定底部、键盘操作、小屏幕、成功新建及取消、切换失败、读取失败回退、保存冲突、迟到快照/搜索/聊天回复/回收预览，以及导航、待办、书签、拓扑、附件、回收站和聊天隔离。
- `TestWorkspaceSwitchScopesKnowledgeAndMetadata`：真实文件和数据库中，在两个工作区反复切换，验证同名笔记、相同查询、待办、附件、图谱链接、书签、聊天历史和长期背景的隔离。
- `navigation-search.cjs`、`view-navigation.cjs`、`task-schedule.cjs`、`graph-relations.cjs`、`trash-browser.cjs`、`editor-attachments.cjs`、`sync-llm.cjs`：相关功能回归。

测试使用隔离目录和模拟接口，不改动用户实际笔记。工作区选择器截图见 [预览](preview-workspace-picker-v0.12.9.png)。

前端类型检查、生产构建及全量 Go 测试通过。最终程序在独立数据目录运行12秒并正常响应；隔离安装、clean PATH 内置 CPU 模型推理、卸载及用户数据保留检查通过。实际用户配置、注册表和快捷方式保持不变。证据见 `.tmp/build-v0129.log`、`.tmp/go-v0129-final.log`、`.tmp/native-v0129-test.log`、`.tmp/installer-v0129-test.log`；校验值见 [SHA-256](release-v0.12.9-sha256.json)。
