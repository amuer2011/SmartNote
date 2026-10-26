export const commands = [
 {id:'syncNow',label:'立即同步当前工作区',key:'',scope:'app'},
 {id:'syncSettings',label:'打开 WebDAV 同步设置',key:'',scope:'app'},
 {id:'llmPolish',label:'打开或关闭 AI 聊天',key:'',scope:'app'},
 {id:'llmSummary',label:'生成检索引用摘要',key:'',scope:'app'},
 {id:'navigationFind',label:'查询导航栏（文件名 / 正文关键词）',key:'',scope:'app'},
 {id:'navigation',label:'打开导航页签',key:'Ctrl+1',scope:'app'},
 {id:'searchTab',label:'打开搜索页签',key:'Ctrl+4',scope:'app'},
 {id:'toggleNoteBookmark',label:'收藏或取消收藏整篇笔记',key:'',scope:'app'},
 {id:'addBookmark',label:'添加当前位置书签',key:'Ctrl+F2',scope:'app'},
 {id:'bookmarks',label:'打开书签',key:'Ctrl+7',scope:'app'},
 {id:'trash',label:'打开回收站页签',key:'Ctrl+8',scope:'app'},
 {id:'organize', label:'智能整理当前笔记', key:'Ctrl+Alt+J', scope:'app'},
 {id:'back', label:'返回源文件 / 后退', key:'Alt+ArrowLeft', scope:'app'},
 {id:'forward', label:'前进到下一篇笔记', key:'Alt+ArrowRight', scope:'app'},
 {id:'newNote', label:'新建笔记', key:'Ctrl+N', scope:'app'},
 {id:'newTab', label:'打开新标签页', key:'Ctrl+T', scope:'app'},
 {id:'nextNoteTab', label:'下一个笔记标签页', key:'Ctrl+Tab', scope:'app'},
 {id:'previousNoteTab', label:'上一个笔记标签页', key:'Ctrl+Shift+Tab', scope:'app'},
 {id:'newFolder', label:'新建文件夹', key:'Ctrl+Shift+N', scope:'app'},
 {id:'openWorkspace', label:'打开 / 添加工作区', key:'Ctrl+O', scope:'app'},
 {id:'save', label:'保存笔记', key:'Ctrl+S', scope:'app'},
 {id:'search', label:'搜索笔记（备用）', key:'Ctrl+K', scope:'app'},
 {id:'globalReplace',label:'工作区查找与替换',key:'Ctrl+Shift+R',scope:'app'},
 {id:'globalSearch', label:'工作区全文搜索', key:'Ctrl+Shift+F', scope:'app'},
 {id:'quickOpen', label:'快速打开笔记', key:'Ctrl+P', scope:'app'},
 {id:'palette', label:'命令面板', key:'Ctrl+Shift+P', scope:'app'},
 {id:'settings', label:'打开设置', key:'Ctrl+,', scope:'app'},
 {id:'sidebar', label:'显示 / 隐藏侧栏', key:'Ctrl+Shift+L', scope:'app'},
 {id:'preview', label:'切换编辑 / 预览', key:'Ctrl+E', scope:'app'},
 {id:'split', label:'分屏编辑', key:'Ctrl+\\', scope:'app'},
 {id:'refresh', label:'刷新工作区', key:'F5', scope:'app'},
 {id:'closeNote', label:'关闭当前笔记', key:'Ctrl+W', scope:'app'},
 {id:'attach', label:'插入附件', key:'Ctrl+Shift+A', scope:'app'},
 {id:'export', label:'导出 Markdown ZIP', key:'Ctrl+Shift+E', scope:'app'},
 {id:'backup', label:'创建原生备份', key:'Ctrl+Alt+B', scope:'app'},
 {id:'import', label:'导入笔记 / 恢复备份', key:'Ctrl+Shift+O', scope:'app'},
 {id:'theme', label:'切换浅色 / 深色', key:'Ctrl+Alt+D', scope:'app'},
 {id:'tasks', label:'打开待办事项', key:'Ctrl+3', scope:'app'},
 {id:'graph', label:'打开知识拓扑', key:'Ctrl+5', scope:'app'},
 {id:'attachments', label:'打开附件库', key:'Ctrl+6', scope:'app'},
 {id:'rename', label:'重命名文件 / 文件夹', key:'F2', scope:'explorer'},
 {id:'delete', label:'移入回收目录', key:'Delete', scope:'explorer'},
 {id:'copyEntry', label:'复制文件 / 文件夹', key:'Ctrl+C', scope:'explorer'},
 {id:'pasteEntry', label:'粘贴文件 / 文件夹副本', key:'Ctrl+V', scope:'explorer'},
 {id:'copyPath', label:'复制相对路径', key:'Ctrl+Alt+C', scope:'explorer'},
 {id:'reveal', label:'在资源管理器中显示', key:'Ctrl+Alt+R', scope:'explorer'},
 {id:'contextMenu', label:'打开文件右键菜单', key:'Shift+F10', scope:'explorer'},
 {id:'undo', label:'撤销编辑', key:'Ctrl+Z', scope:'editor'},
 {id:'redo', label:'重做编辑', key:'Ctrl+Y', scope:'editor'},
 {id:'redoAlternate', label:'重做编辑（备用）', key:'Ctrl+Shift+Z', scope:'editor'},
 {id:'find', label:'查找当前笔记', key:'Ctrl+F', scope:'app'},
 {id:'replace', label:'替换当前笔记', key:'Ctrl+R', scope:'app'},
 {id:'selectAll', label:'全选正文', key:'Ctrl+A', scope:'editor'},
 {id:'copy', label:'复制正文', key:'Ctrl+C', scope:'editor'},
 {id:'cut', label:'剪切正文', key:'Ctrl+X', scope:'editor'},
 {id:'paste', label:'粘贴正文', key:'Ctrl+V', scope:'editor'},
 {id:'bold', label:'加粗', key:'Ctrl+B', scope:'editor'},
 {id:'italic', label:'斜体', key:'Ctrl+I', scope:'editor'},
 {id:'inlineCode', label:'行内代码', key:'Ctrl+`', scope:'editor'},
 {id:'link', label:'插入链接', key:'Ctrl+L', scope:'editor'},
 {id:'strike',label:'删除线',key:'',scope:'editor'},
 {id:'heading',label:'设置标题',key:'',scope:'editor'},
 {id:'bulletList',label:'无序列表',key:'',scope:'editor'},
 {id:'orderedList',label:'有序列表',key:'',scope:'editor'},
 {id:'quote',label:'引用',key:'',scope:'editor'},
 {id:'codeBlock',label:'插入代码块',key:'',scope:'editor'},
 {id:'table',label:'插入表格',key:'',scope:'editor'},
 {id:'horizontalRule',label:'插入分隔线',key:'',scope:'editor'},
 {id:'clearFormatting',label:'清除格式',key:'',scope:'editor'},
] as const
export type CommandId = typeof commands[number]['id']
export type ShortcutMap = Partial<Record<CommandId,string>>
export function binding(id:CommandId, overrides:Record<string,string> = {}) {
 return overrides[id] ?? commands.find(command=>command.id===id)!.key
}
const aliases:Record<string,string> = { ' ':'Space', del:'Delete', esc:'Escape', arrowup:'ArrowUp', arrowdown:'ArrowDown', arrowleft:'ArrowLeft', arrowright:'ArrowRight', escape:'Escape', delete:'Delete', backspace:'Backspace', enter:'Enter', tab:'Tab', insert:'Insert', home:'Home', end:'End', pageup:'PageUp', pagedown:'PageDown' }
export function keyStroke(event:KeyboardEvent):string {
 if(event.isComposing || ['Control','Shift','Alt','Meta','Process','Dead'].includes(event.key)) return ''
 const key=aliases[event.key.toLowerCase()] || (/^f\d{1,2}$/i.test(event.key)?event.key.toUpperCase():event.key.length===1?event.key.toUpperCase():event.key)
 return [...(event.ctrlKey||event.metaKey?['Ctrl']:[]),...(event.altKey?['Alt']:[]),...(event.shiftKey?['Shift']:[]),key].join('+')
}
export function shortcutIssue(id:CommandId,key:string,overrides:Record<string,string>):string {
 if(!key) return ''
 const command=commands.find(item=>item.id===id)!
 const main=key.split('+').pop()!
 if(!/^(Ctrl|Alt)\+/.test(key) && !/^F([1-9]|1[0-2])$/.test(main) && !(command.scope==='explorer' && ['Delete','Backspace'].includes(main))) return '请使用 Ctrl / Alt 组合键或 F1–F12，避免覆盖文字输入。'
 if(['Escape','Enter'].includes(main) || main==='Tab'&&!/^Ctrl\+(Shift\+)?Tab$/.test(key) || ['Alt+F4','Ctrl+Alt+DELETE','Ctrl+Alt+Delete'].includes(key)) return '此按键由系统或界面保留。'
 const conflict=commands.find(item=>item.id!==id && binding(item.id,overrides)===key && (item.scope===command.scope || item.scope==='app' || command.scope==='app'))
 return conflict?`与“${conflict.label}”冲突，请选择其他按键。`:''
}

export const isMac = typeof navigator!=='undefined' && /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)
export function formatShortcut(key:string,mac=isMac){return mac?key.replace(/\bCtrl\b/g,'Command').replace(/\bAlt\b/g,'Option'):key}
export function accessibleShortcut(key:string){return key.replace(/\bCtrl\b/g,isMac?'Meta':'Control')}
