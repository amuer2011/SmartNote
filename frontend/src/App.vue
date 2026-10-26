<script setup lang="ts">
import { version as appVersion } from '../package.json'

import { warmCalendar } from './services/calendar-cache'

import { CalendarDays } from 'lucide-vue-next'

import { computed, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'

const semanticConfigRevision=ref(0)

import { NConfigProvider, NButton, NInput, NSelect, NSwitch, darkTheme, zhCN, dateZhCN } from 'naive-ui'

import { LocateFixed, ChevronsDownUp, ChevronsUpDown as ExpandDirectories, Bookmark as BookmarkIcon, BookOpen, Search, ListTodo, Network, Paperclip, Settings2, Plus, ChevronDown, ChevronRight, Folder, FileText, PanelLeftClose, Columns2, Eye, Code2, MoreHorizontal, Download, Upload, ArrowUpRight, Check, Circle, X, Sun, Moon, Command, Leaf, ShieldCheck, HardDrive, Sparkles, ArrowLeft, ArrowRight, Copy, Trash2, ChevronsUpDown } from 'lucide-vue-next'

import { useWorkspace } from './stores/workspace'

import {readNoteSession,rememberNoteSession,clearNoteSession} from './services/note-session'

import { api, desktop, type Note, type SearchResult, type Edge, type Attachment, type UIPreferences, type LayoutPreferences, type ArchiveResult, type Bookmark, type GraphAnnotations } from './services/api'

import { startNoteDrag } from './services/note-drag'

import {bookmarkLine} from './services/bookmarks'

import MarkdownEditor from './components/MarkdownEditor.vue'
import MarkdownToolbar from './components/MarkdownToolbar.vue'
import NavigationSearch from './components/NavigationSearch.vue'
import AttachmentContent from './components/AttachmentContent.vue'
import ImageViewer from './components/ImageViewer.vue'
import AttachmentLibrary from './components/AttachmentLibrary.vue'
import {editorDocument,noteTimestamp} from './services/editor-document'
import type {AttachmentPreview} from './services/api'
const metadataVisible=ref(false)

import NoteFindBar from './components/NoteFindBar.vue'

import SyncSettings from './components/SyncSettings.vue'
import SyncIndicator from './components/SyncIndicator.vue'
import LLMSettings from './components/LLMSettings.vue'
import AIAssistant from './components/AIAssistant.vue'
import LLMReview from './components/LLMReview.vue'
import type {LLMInput,LLMResult,LLMSource,LLMEditProposal} from './services/api'
import IndexSettings from './components/IndexSettings.vue'

import ReminderCenter from './components/ReminderCenter.vue'

import ReminderSettings from './components/ReminderSettings.vue'

const reminderRevision=ref(0)

import WorkspaceTextSearch from './components/WorkspaceTextSearch.vue'

import {textMatches,applyMatches} from './services/text-search'

import MarkdownPreview from './components/MarkdownPreview.vue'

import KnowledgeGraph from './components/KnowledgeGraph.vue'
import GraphRelations from './components/GraphRelations.vue'

import TodoView from './components/TodoView.vue'

import WorkspaceTree from './components/WorkspaceTree.vue'
import RightPanelTabs from './components/RightPanelTabs.vue'
import NoteTabs from './components/NoteTabs.vue'
import {contextMenuPosition} from './services/context-menu'
import WorkspacePicker from './components/WorkspacePicker.vue'
import FolderImportDialog from './components/FolderImportDialog.vue'
const folderImportOpen=ref(false)
import TrashBrowser from './components/TrashBrowser.vue'
import type {TrashPreview} from './services/trash-tree'
const trashPreview=ref<TrashPreview>()
import TaskIntentConfirmation from './components/TaskIntentConfirmation.vue'

import DirectoryPicker from './components/DirectoryPicker.vue'

import ArchiveRules from './components/ArchiveRules.vue'

import SemanticSettings from './components/SemanticSettings.vue'

import ArchivePanel from './components/ArchivePanel.vue'

import FloatingWindow from './components/FloatingWindow.vue'

import ResizableModal from './components/ResizableModal.vue'

import PanelDivider from './components/PanelDivider.vue'

import { commands, binding, keyStroke, shortcutIssue, formatShortcut, accessibleShortcut, type CommandId } from './services/shortcuts'

import { fontOptions, fontStack } from './services/fonts'

const backend = () => api

async function copyDraft() { await navigator.clipboard.writeText(store.content) }

const store = useWorkspace()

const actualDataDirectory = ref('')

const noteSessionReady=ref(false)
let startupRestoreCancelled=false
watch([noteSessionReady,()=>store.switching,()=>store.workspace?.id,()=>store.active?.id,()=>store.active?.path],()=>{
 if(noteSessionReady.value&&!store.switching&&store.workspace&&store.active)rememberNoteSession(actualDataDirectory.value,store.workspace.id,store.active)
},{flush:'sync'})

type Page = 'notes' | 'tasks' | 'search' | 'graph' | 'attachments'

const page = ref<Page>('notes'), folder = ref(''), filter = ref(''), query = ref(''), results = ref<SearchResult[]>([]), searched = ref(false), searching = ref(false)

const selectedEntry = ref('')

const mode = ref('preview'), line = ref<number>()

const modal = ref<'note' | 'delete' | 'conflict' | 'remove' | 'rename' | 'move' | 'entryRename' | 'entryDelete' | 'folder' | 'palette' | 'quickOpen' | null>(null), noteTitle = ref(''), noteDir = ref('Inbox'), rename = ref(''), movePath = ref('')

const settingsTab = ref('general'), picgo = ref(''), busy = ref(false), toast = ref(''), edges = ref<Edge[]>([]), globalGraph = ref(false), relationFilter = ref('')

const todo=ref<InstanceType<typeof TodoView>>()

const taskView=ref('day')

const editor = ref<InstanceType<typeof MarkdownEditor>>(), fileInput = ref<HTMLInputElement>()

type RightTab='outline'|'ai'|'related'
const rightTab=ref<RightTab>('outline'),aiSelection=ref(''),llmSettingsRevision=ref(0)
const allRightTabs=[{id:'outline',label:'大纲',icon:ListTodo},{id:'related',label:'关联',icon:Network},{id:'ai',label:'AI 聊天',icon:Sparkles}] as const
const orderedRightTabs=computed(()=>{const order=[...new Set([...(ui.value.layout?.rightTabOrder||[]),'outline','related','ai'])];return order.map(id=>allRightTabs.find(tab=>tab.id===id)).filter((tab):tab is typeof allRightTabs[number]=>!!tab)})
const rightTabs=computed(()=>orderedRightTabs.value.filter(tab=>!ui.value.layout?.hiddenRightTabs?.includes(tab.id)))
function closeRightTab(id:string){layout.value.hiddenRightTabs=[...new Set([...(layout.value.hiddenRightTabs||[]),id])];if(rightTab.value===id)rightTab.value=rightTabs.value[0]?.id||'outline';persistUI()}
function openRightTab(id:string){layout.value.hiddenRightTabs=(layout.value.hiddenRightTabs||[]).filter(tab=>tab!==id);rightTab.value=id as RightTab;setPanel('inspector',true)}
function showAllRightTabs(){layout.value.hiddenRightTabs=[];if(!store.active)rightTab.value='ai';setPanel('inspector',true);layoutOpen.value=false}
function reorderRightTabs(ids:string[]){layout.value.rightTabOrder=[...ids,...orderedRightTabs.value.map(t=>t.id).filter(id=>!ids.includes(id))];persistUI()}
const aiOpen=computed({get:()=>rightTab.value==='ai'&&!isHidden('inspector')&&rightTabs.value.some(tab=>tab.id==='ai'),set:(open:boolean)=>{if(open)openRightTab('ai');else{rightTab.value=rightTabs.value.find(tab=>tab.id!=='ai')?.id||'outline';if(!rightTabs.value.some(tab=>tab.id===rightTab.value))closeRightTab('ai')}}})
const rightPanelVisible=computed(()=>!!rightTabs.value.length&&!isHidden('inspector')&&(rightTab.value==='ai'||!!store.active&&!!activeInDirectory.value))
function toggleAI(){
 if(aiOpen.value){aiOpen.value=false;return}
 const native=window.getSelection(),inPreview=!!native?.anchorNode&&!!document.querySelector('.preview-pane')?.contains(native.anchorNode)
 aiSelection.value=(inPreview?native?.toString():editor.value?.selectedText()?.text)||''
 aiOpen.value=true
}
async function prepareAIContext(){
 const workspaceID=store.workspace?.id,noteID=store.active?.id
 await store.save()
 if(store.dirty||store.conflict||workspaceID!==store.workspace?.id||noteID!==store.active?.id)throw Error('请先处理未保存内容或笔记变化，再发送消息。')
 return {noteId:store.active?.id||'',hash:store.active?.hash||''}
}
async function applyAIEdit(proposal:LLMEditProposal){
 if(store.workspace?.id!==proposal.workspaceId)throw Error('工作区已变化，未应用修改。')
 await store.save()
 if(store.dirty||store.conflict||store.workspace?.id!==proposal.workspaceId)throw Error('请先处理未保存内容。')
 await api.ApplyLLMEdit(proposal)
 aiSelection.value=''
 if(store.workspace?.id===proposal.workspaceId)await store.refresh()
 notify('已确认保存 AI 修改')
}
async function openAISource(source:LLMSource){const note=store.data.notes.find(n=>n.id===source.noteId);if(note)await openNote(note,source.line);else notify('来源笔记已删除，请重新检索')}
watch(()=>store.active?.id,()=>{aiSelection.value=''})
const llmOpen=ref(false),llmInput=ref<LLMInput|null>(null)
let llmDocument=''
async function prepareLLM(action:LLMInput['action']){
 const workspaceId=store.workspace?.id;if(!workspaceId)return
 const nativeSelection=window.getSelection(),inPreview=!!nativeSelection?.anchorNode&&!!document.querySelector('.preview-pane')?.contains(nativeSelection.anchorNode)
 let selection=sourceVisible.value&&!inPreview?editor.value?.selectedText():undefined
 if(action!=='summary'&&(!selection?.text)){
  const selected=window.getSelection(),node=selected?.anchorNode
  const text=selected?.toString()||''
  if(text&&node&&document.querySelector('.preview-pane')?.contains(node)){
   const from=store.content.indexOf(text)
   if(from>=0&&store.content.indexOf(text,from+1)<0)selection={from,to:from+text.length,text}
  }
 }
 if(action!=='summary'&&!selection?.text){notify('请先在笔记中选择需要处理的文本；复杂格式可在编辑器中选择');return}
 const noteId=store.active?.id,documentBefore=store.content
 try{
  await store.save()
  if(store.dirty||store.workspace?.id!==workspaceId||store.active?.id!==noteId){notify('请先处理未保存内容或工作区变化');return}
  const requestId=crypto.randomUUID()
  if(action==='summary'){
   const refs=results.value.slice(0,8).map(r=>({noteId:r.note.id,hash:r.note.hash,title:r.note.title,path:r.note.path,line:r.line}))
   if(!refs.length)return
   llmInput.value={requestId,workspaceId,action,query:query.value,sources:refs}
  }else{
   if(!store.active||store.content!==documentBefore||!selection){notify('保存过程中笔记发生变化，请重新选择文本');return}
   llmDocument=store.content
   llmInput.value={requestId,workspaceId,action,noteId:store.active.id,hash:store.active.hash,...selection}
  }
  llmOpen.value=true
 }catch(e){notify(String(e))}
}
function applyLLM(result:LLMResult){
 const input=llmInput.value
 if(!input||input.action==='summary'||store.workspace?.id!==input.workspaceId||store.active?.id!==input.noteId||store.content!==llmDocument||store.conflict||result.original!==input.text){notify('笔记已变化，未替换内容。可复制结果后手动合并。');return}
 const from=input.from!,to=input.to!
 if(store.content.slice(from,to)!==result.original){notify('所选位置已变化，请重新生成');return}
 if(!editor.value?.replaceRange(from,to,result.original,result.text))store.edit(store.content.slice(0,from)+result.text+store.content.slice(to))
 llmOpen.value=false;notify('已替换所选文本，可继续编辑')
}
async function openLLMSource(source:LLMSource){
 if(store.workspace?.id!==llmInput.value?.workspaceId){notify('工作区已变化，请重新搜索');return}
 const note=store.data.notes.find(n=>n.id===source.noteId)
 if(note){await openNote(note,source.line);llmOpen.value=false}else notify('来源笔记已删除，请重新搜索')
}


const hiddenImage=ref('')

const image = ref('')

let unlisten: (() => void) | undefined; let timer: ReturnType<typeof setTimeout> | undefined; let noticeTimer: ReturnType<typeof setTimeout>

const nav = [ { id: 'tasks', label: '待办事项', icon: ListTodo }, { id: 'search', label: '搜索', icon: Search }, { id: 'graph', label: '知识拓扑', icon: Network }, { id: 'attachments', label: '附件库', icon: Paperclip } ] as const

type SidebarTab=Exclude<Page,'notes'>|'navigation'|'bookmarks'|'trash'

const sidebarTab=ref<SidebarTab>('navigation')

const sidebarTabs=[{id:'navigation',label:'导航',icon:Folder},...nav,{id:'bookmarks',label:'书签',icon:BookmarkIcon},{id:'trash',label:'回收站',icon:Trash2}] as const



function selectSidebarTab(id:SidebarTab){if(id==='graph'){openGraph();return};if(graphOpen.value)closeGraph();collapsed.value=false;if(id==='tasks')taskView.value='day';sidebarTab.value=id;page.value=id==='navigation'||id==='bookmarks'||id==='trash'?'notes':id}

function sidebarTabKey(event:KeyboardEvent){

 if(!['ArrowUp','ArrowDown','Home','End'].includes(event.key))return

 const index=sidebarTabs.findIndex(tab=>tab.id===sidebarTab.value)

 const next=event.key==='Home'?0:event.key==='End'?sidebarTabs.length-1:(index+(event.key==='ArrowDown'?1:-1)+sidebarTabs.length)%sidebarTabs.length

 event.preventDefault();selectSidebarTab(sidebarTabs[next].id);nextTick(()=>document.getElementById('sidebar-tab-'+sidebarTabs[next].id)?.focus())

}

watch(page,value=>{if(value!=='notes')sidebarTab.value=value})

const bookmarks=ref<Bookmark[]>([]),bookmarkName=ref(''),bookmarkFilter=ref(''),bookmarkBusy=ref(false),bookmarkError=ref('')

let bookmarkSequence=0

const visibleBookmarks=computed(()=>bookmarks.value.filter(item=>{

 const note=store.data.notes.find(note=>note.id===item.noteId)

 return !bookmarkFilter.value || (item.label+' '+(note?.title||'')+' '+item.text).toLowerCase().includes(bookmarkFilter.value.toLowerCase())

}))

async function refreshBookmarks(){

 const id=store.workspace?.id,seq=++bookmarkSequence

 bookmarks.value=[];bookmarkError.value=''

 if(!id||!desktop)return

 try{const values=await api.GetBookmarks(id);if(seq===bookmarkSequence)bookmarks.value=values||[]}catch(e){if(seq===bookmarkSequence)bookmarkError.value=String(e)}

}

watch(()=>store.workspace?.id,()=>void refreshBookmarks(),{immediate:true})

const activeNoteBookmark=computed(()=>bookmarks.value.find(item=>item.noteId===store.active?.id&&item.kind==='note'))

async function toggleNoteBookmark(){
 if(bookmarkBusy.value)return
 const id=store.workspace?.id,note=store.active;if(!id||!note)return
 bookmarkBusy.value=true;bookmarkError.value=''
 try{
  const existing=activeNoteBookmark.value
  if(existing){await api.DeleteBookmark(id,existing.id);notify('已取消整篇笔记收藏')}
  else{
   await store.save()
   if(store.dirty||store.workspace?.id!==id||store.active?.id!==note.id)throw Error('请先保存当前笔记后再收藏')
   await api.AddBookmark({workspaceId:id,noteId:note.id,hash:store.active.hash,kind:'note',line:0,column:0,label:note.title});notify('已收藏整篇笔记')
  }
  if(store.workspace?.id===id)await refreshBookmarks()
 }catch(e){bookmarkError.value=String(e);notify(String(e))}finally{bookmarkBusy.value=false}
}

async function addBookmark(){

 if(bookmarkBusy.value)return

 const id=store.workspace?.id,note=store.active,position=editor.value?.bookmarkPosition()

 if(!id||!note||!position){notify('请先在编辑器中将光标放到要收藏的位置');return}

 bookmarkBusy.value=true;bookmarkError.value=''

 try{

  await store.save()

  if(store.dirty||store.workspace?.id!==id||store.active?.id!==note.id)throw Error('请先保存当前笔记后再添加书签')

  const target=bookmarkLine(store.active.content,position)

  await api.AddBookmark({workspaceId:id,noteId:note.id,hash:store.active.hash,kind:'position',line:target.line,column:position.column,label:bookmarkName.value})

  bookmarkName.value='';await refreshBookmarks();notify('已添加当前位置书签')

 }catch(e){bookmarkError.value=String(e);notify(String(e))}finally{bookmarkBusy.value=false}

}

async function visitBookmark(item:Bookmark){

 const note=store.data.notes.find(note=>note.id===item.noteId)

 if(!note){notify('书签对应的笔记已删除或不在当前工作区');return}

 if(!await openNote(note,item.kind==='note'?0:undefined))return

 if(item.kind==='note')return

 setMode(mode.value==='preview'?'split':mode.value)

 await nextTick()

 const target=bookmarkLine(store.content,item)

 editor.value?.jumpToPosition(target.line,item.column)

 if(!target.found)notify('原书签内容已变化，已跳到保存的行号附近')

}

async function removeBookmark(item:Bookmark){

 const id=store.workspace?.id;if(!id)return

 try{await api.DeleteBookmark(id,item.id);await refreshBookmarks()}catch(e){bookmarkError.value=String(e)}

}

const settingsOpen=ref(false),layoutOpen=ref(false)

const archiveOpen=ref(false),archiveMinimized=ref(false),lastArchive=ref<{workspaceId:string;result:ArchiveResult}>()

const defaultLayout=():LayoutPreferences=>({sidebarWidth:244,listWidth:292,inspectorWidth:340,splitRatio:50,hiddenPanels:[]})

const ui = ref<UIPreferences>({layout:defaultLayout(),fontSize:14,fontFamily:'system',editorFontFamily:'system',editorFontSize:14,shortcuts:{},collapsedWorkspaces:[],entryOrder:{}})

let uiSave = Promise.resolve()

let pendingUISaves = 0

watch(() => store.data.settings.ui, value => { if(value && !pendingUISaves) ui.value={layout:{...defaultLayout(),...value.layout,sidebarWidth:value.layout?.sidebarWidth || 244,listWidth:value.layout?.listWidth || 292,inspectorWidth:value.layout?.inspectorWidth || 340,splitRatio:value.layout?.splitRatio || 50,hiddenPanels:value.layout?.hiddenPanels || []},fontSize:value.fontSize || 14,fontFamily:value.fontFamily || 'system',editorFontFamily:value.editorFontFamily || value.fontFamily || 'system',editorFontSize:value.editorFontSize || 14,shortcuts:value.shortcuts || {},collapsedWorkspaces:value.collapsedWorkspaces || [],entryOrder:value.entryOrder || {}} }, {immediate:true})

watch(() => ui.value.fontSize, value => { document.documentElement.style.fontSize = value + 'px' }, {immediate:true})

watch(() => ui.value.fontFamily, value => { document.documentElement.style.setProperty('--ui-font-family',fontStack(value)) }, {immediate:true})

watch(() => ui.value.editorFontFamily, value => { document.documentElement.style.setProperty('--note-font-family',fontStack(value)) }, {immediate:true})

watch(() => ui.value.editorFontSize, value => { document.documentElement.style.setProperty('--note-font-size',(value || 14)+'px') }, {immediate:true})

function persistUI() { const snapshot=JSON.parse(JSON.stringify(ui.value)) as UIPreferences; pendingUISaves++; uiSave=uiSave.then(() => api.SetUIPreferences(snapshot)).catch(e=>store.report(e)).finally(()=>pendingUISaves--) }

type PanelId='sidebar'|'list'|'editor'|'preview'|'inspector'

const panels=[{id:'sidebar',label:'导航栏'},{id:'list',label:'笔记列表'},{id:'editor',label:'编辑器'},{id:'preview',label:'预览'},{id:'inspector',label:'右侧栏（大纲 / 关联 / AI 聊天）'}] as const

const layout=computed(()=>ui.value.layout!)
watch(rightTabs,tabs=>{if(tabs.length&&!tabs.some(tab=>tab.id===rightTab.value))rightTab.value=tabs[0]!.id},{immediate:true})
const notesLayout=ref<HTMLElement>(),notesLayoutWidth=ref(window.innerWidth)
const inspectorDisplayWidth=computed(()=>Math.max(0,Math.min(layout.value.inspectorWidth,notesLayoutWidth.value-6)))
let notesLayoutObserver:ResizeObserver|undefined
watch(notesLayout,element=>{notesLayoutObserver?.disconnect();if(!element)return;notesLayoutWidth.value=element.getBoundingClientRect().width;notesLayoutObserver=new ResizeObserver(()=>{notesLayoutWidth.value=element.getBoundingClientRect().width});notesLayoutObserver.observe(element)},{flush:'post'})
onBeforeUnmount(()=>notesLayoutObserver?.disconnect())


const isHidden=(id:PanelId)=>layout.value.hiddenPanels.includes(id)

const collapsed=computed({get:()=>isHidden('sidebar'),set:(value:boolean)=>setPanel('sidebar',!value)})

const sourceVisible=computed(()=>mode.value!=='preview' && !isHidden('editor'))

const previewVisible=computed(()=>mode.value!=='edit' && !isHidden('preview'))

function panelVisible(id:PanelId) {return id==='editor'?sourceVisible.value:id==='preview'?previewVisible.value:id==='inspector'?!!rightTabs.value.length&&!isHidden(id):!isHidden(id)}

function setPanel(id:PanelId,visible:boolean) {

 layout.value.hiddenPanels=visible?layout.value.hiddenPanels.filter(panel=>panel!==id):[...new Set([...layout.value.hiddenPanels,id])]

 if(visible && id==='editor' && mode.value==='preview') mode.value='split'

 if(visible && id==='preview' && mode.value==='edit') mode.value='split'

 persistUI()

}

function setMode(value:string) {

 mode.value=value

 const show=value==='split'?['editor','preview']:value==='edit'?['editor']:['preview']

 layout.value.hiddenPanels=layout.value.hiddenPanels.filter(panel=>!show.includes(panel)); persistUI()

}

function resetLayout() {ui.value.layout=defaultLayout();rightTab.value='outline'; mode.value='split'; persistUI()}

function resizePanel(id:'sidebar'|'list'|'inspector'|'split',delta:number) {

 const selectors={sidebar:'.sidebar',list:'.note-list-panel',inspector:'.inspector',split:'.source-pane'}

 const actual=document.querySelector(selectors[id])?.getBoundingClientRect().width

 const width=document.querySelector('.notes-layout')?.getBoundingClientRect().width || window.innerWidth

 const clamp=(n:number,min:number,max:number)=>Math.round(Math.max(min,Math.min(max,n)))

 if(id==='sidebar') layout.value.sidebarWidth=clamp((actual || layout.value.sidebarWidth)+delta,180,Math.min(480,window.innerWidth*.3))

 if(id==='list') layout.value.listWidth=clamp((actual || layout.value.listWidth)+delta,160,Math.min(600,width*.4))

 if(id==='inspector') layout.value.inspectorWidth=clamp((actual || layout.value.inspectorWidth)-delta,Math.min(120,Math.max(0,width-6)),Math.max(0,width-6))

 if(id==='split') {const total=document.querySelector('.editor-body')?.getBoundingClientRect().width || 1; layout.value.splitRatio=clamp(layout.value.splitRatio+delta/Math.max(1,total-6)*100,15,85)}

}

function editorWheel(event:WheelEvent) {

 if(!(event.ctrlKey || event.metaKey) || !event.deltaY || !(event.target instanceof Element) || !event.target.closest('.editor-body')) return

 event.preventDefault(); event.stopPropagation()

 const next=Math.max(10,Math.min(48,(ui.value.editorFontSize || 14)+(event.deltaY<0?1:-1)))

 if(next!==ui.value.editorFontSize) setEditorFontSize(next)

}

function setFontSize(value:number) { ui.value.fontSize=value; persistUI() }

function setEditorFontFamily(value:string) { ui.value.editorFontFamily=value; persistUI() }

function setEditorFontSize(value:number) { ui.value.editorFontSize=value; persistUI() }

function setFontFamily(value:string) { ui.value.fontFamily=value; persistUI() }

const navigationSearchOpen=ref(false),navigationQuery=ref(''),navigationSearch=ref<InstanceType<typeof NavigationSearch>>()
const navigationFiles=computed(()=>({notes:store.data.notes.map(note=>note.id===store.active?.id?{...note,content:store.content}:note),directories:store.data.directories||[]}))

async function openNavigationSearch(){selectSidebarTab('navigation');navigationSearchOpen.value=true;await nextTick();navigationSearch.value?.focus()}
function closeNavigationSearch(){navigationSearchOpen.value=false;navigationQuery.value='';nextTick(()=>document.getElementById('sidebar-panel-navigation')?.focus({preventScroll:true}))}

const treeInstances=new Map<string,InstanceType<typeof WorkspaceTree>>()
function setTreeRef(id:string,instance:unknown){if(instance)treeInstances.set(id,instance as InstanceType<typeof WorkspaceTree>);else treeInstances.delete(id)}
async function locateCurrentNote(){
 const note=store.active;if(!note||!store.workspace)return
 selectSidebarTab('navigation');ui.value.collapsedWorkspaces=ui.value.collapsedWorkspaces.filter(id=>id!==store.workspace!.id);persistUI()
 selectedEntry.value='file:'+note.id
 await nextTick();await treeInstances.get(store.workspace.id)?.reveal(note.path,{focus:true})
}
async function expandDirectories(){ui.value.collapsedWorkspaces=[];persistUI();await nextTick();for(const tree of treeInstances.values())tree.expandAll()}
function collapseDirectories(){for(const tree of treeInstances.values())tree.collapseAll()}
function toggleWorkspace(id:string) { const closed=ui.value.collapsedWorkspaces; ui.value.collapsedWorkspaces=closed.includes(id)?closed.filter(item=>item!==id):[...closed,id]; persistUI() }

function orderEntries(id:string,parent:string,paths:string[]) { ui.value.entryOrder={...ui.value.entryOrder,[id]:{...ui.value.entryOrder[id],[parent]:paths}}; persistUI() }

const folders = computed(() => {

 const set = new Set(store.data.directories || [])

 for (const note of store.data.notes) {

  const parts = note.path.split('/'); parts.pop()

  while (parts.length) { set.add(parts.join('/')); parts.pop() }

 }

 return [...set].sort((a, b) => a.localeCompare(b, 'zh-CN'))

})

const parentDirectory = (path: string) => path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : ''

async function enterListFolder(path:string) {enterFolder(path); await nextTick(); document.querySelector<HTMLElement>('.note-list')?.focus()}

function enterFolder(path: string) { if(store.workspace) treeTarget.value=entryTarget(path,true); folder.value = path || '.'; selectedEntry.value = 'dir:' + folder.value; filter.value = ''; page.value = 'notes' }

async function openTreeNote(note: Note,matchLine?:number,newTab=false) { await openNote(note,matchLine,true,newTab); if (store.active?.id === note.id) { enterFolder(parentDirectory(note.path)); treeTarget.value=entryTarget(note.path,false); selectedEntry.value = 'file:' + note.id } }

watch(() => store.workspace?.id, () => {
 selectedEntry.value='';folder.value='';filter.value='';query.value='';navigationQuery.value=''
 backHistory.value=[];forwardHistory.value=[];archiveOpen.value=false;archiveMinimized.value=false;lastArchive.value=undefined
 trashPreview.value=undefined;image.value='';hiddenImage.value='';modal.value=null
 treeTarget.value=undefined;operationTarget.value=undefined;copiedEntry.value=undefined;contextMenu.value=undefined
 bookmarkFilter.value='';bookmarkName.value='';aiSelection.value='';llmOpen.value=false;llmInput.value=null
 graphOpen.value=false;graphReturn={tab:'navigation',page:'notes'}
 taskView.value='day';line.value=undefined;closeFind();sidebarTab.value='navigation';page.value='notes'
})

const currentDirectory = computed(() => folder.value === '.' ? '' : folder.value)

const notes = computed(() => store.data.notes.filter(n => (!folder.value || parentDirectory(n.path) === currentDirectory.value) && (n.title + n.path + n.tags.join(' ')).toLowerCase().includes(filter.value.toLowerCase())))

const childFolders = computed(() => folder.value ? folders.value.filter(path => parentDirectory(path) === currentDirectory.value && path.toLowerCase().includes(filter.value.toLowerCase())) : [])

const listEntries = computed(() => {

 const entries = [...childFolders.value.map(path=>({path,directory:path,note:undefined as Note|undefined})),...notes.value.map(note=>({path:note.path,directory:'',note}))]

 if (!folder.value) return entries

 const savedOrder=ui.value.entryOrder[store.workspace?.id || '']?.[currentDirectory.value]

 const order=Array.isArray(savedOrder)?savedOrder:[]

 return entries.sort((a,b)=> { const ai=order.indexOf(a.path),bi=order.indexOf(b.path); if(ai>=0 || bi>=0) return (ai<0?Infinity:ai)-(bi<0?Infinity:bi); return Number(!!b.directory)-Number(!!a.directory) || a.path.localeCompare(b.path,'zh-CN') })

})

const activeInDirectory = computed(() => store.active && (!folder.value || parentDirectory(store.active.path) === currentDirectory.value))

const previewRef=ref<InstanceType<typeof MarkdownPreview>>(),findBar=ref<InstanceType<typeof NoteFindBar>>()

const findOpen=ref(false),findQuery=ref(''),findCount=ref(0),findIndex=ref(0),findTarget=ref<'editor'|'preview'>('editor')

const findRegex=ref(false),findCase=ref(false),findReplacement=ref(''),findReplaceOpen=ref(false),findError=ref('')

const activeFindTarget=computed(()=>!sourceVisible.value?'preview':!previewVisible.value?'editor':findTarget.value)

async function updateFind(reset=false){

 if(reset)findIndex.value=0

 await nextTick();findError.value=''

 try{

 const term=findOpen.value?findQuery.value:''

 textMatches('',term,findRegex.value,findCase.value)

 if(activeFindTarget.value==='preview'){editor.value?.findMatches('');findCount.value=previewRef.value?.findMatches(term,findIndex.value,findRegex.value,findCase.value)||0}

 else{previewRef.value?.findMatches('');findCount.value=editor.value?.findMatches(term,findIndex.value,findRegex.value,findCase.value)||0}

 if(findReplaceOpen.value)findCount.value=textMatches(noteSearchDocument.value.body,term,findRegex.value,findCase.value).length

 if(findCount.value)findIndex.value=((findIndex.value%findCount.value)+findCount.value)%findCount.value

 }catch(e){findCount.value=0;findError.value='正则表达式无效：'+String(e);editor.value?.findMatches('');previewRef.value?.findMatches('')}

}

function replaceInNote(all:boolean){

 try{

 const matches=textMatches(noteSearchDocument.value.body,findQuery.value,findRegex.value,findCase.value,findReplacement.value)

 if(!matches.length)return

 const selected=all?matches:[matches[findIndex.value%matches.length]]

 const updated=noteSearchDocument.value.prefix+applyMatches(noteSearchDocument.value.body,selected);store.edit(updated)

 if(all)findIndex.value=0

 void updateFind();notify(`已替换 ${selected.length} 处`)

 }catch(e){findError.value=String(e)}

}

async function openFind(){if(!store.active||!activeInDirectory.value){notify('请先打开一篇笔记，再使用页面内查找');return};findOpen.value=true;await updateFind();findBar.value?.focus()}

function closeFind(){findOpen.value=false;void updateFind()}

function stepFind(delta:number){if(findCount.value)findIndex.value=(findIndex.value+delta+findCount.value)%findCount.value;void updateFind()}

watch([findQuery,findRegex,findCase,findReplaceOpen],()=>void updateFind(true))

watch([activeFindTarget,metadataVisible,()=>store.content],()=>{if(findOpen.value)void updateFind()})

watch(()=>store.active?.id,()=>{findOpen.value=false;findQuery.value='';findIndex.value=0})

async function jumpOutline(target:number){await nextTick();if(sourceVisible.value)editor.value?.jumpToPosition(target);if(previewVisible.value)previewRef.value?.scrollToLine(target)}

const graphOpen=ref(false),graphInitialized=ref(false),graphView=ref<InstanceType<typeof KnowledgeGraph>>()
const inspectedGraphEdge=ref<Edge>(),graphRelationsVisible=ref(false),inspectedGraphNode=ref<string>(),graphLinking=ref(false),graphPair=ref<{source:string;target:string}>()
const graphAnnotations=ref<GraphAnnotations>({revision:'',nodes:{},relations:{}})
const graphEdges=computed(()=>edges.value.filter(e=>['MARKDOWN_LINK','USER_CONFIRMED'].includes(e.type)))
function inspectGraphNode(id:string){inspectedGraphNode.value=id;inspectedGraphEdge.value=undefined;graphPair.value=undefined}
function inspectGraphEdge(edge:Edge){inspectedGraphEdge.value=edge;inspectedGraphNode.value=undefined;graphPair.value=undefined}
function connectGraph(pair:{source:string;target:string}){graphPair.value=pair;inspectedGraphNode.value=undefined;inspectedGraphEdge.value=undefined;graphRelationsVisible.value=true;graphLinking.value=false}
function manualGraph(){graphRelationsVisible.value=!graphRelationsVisible.value;inspectedGraphNode.value=undefined;inspectedGraphEdge.value=undefined;graphPair.value=undefined}
let graphRequest=0
async function refreshGraph(){
 const request=++graphRequest,workspace=store.workspace?.id
 if(!desktop||!workspace){edges.value=[];return}
 try{const result=await api.Graph();if(request===graphRequest&&workspace===store.workspace?.id)edges.value=result||[];const metadata=await api.GetGraphAnnotations(workspace);if(request===graphRequest&&workspace===store.workspace?.id)graphAnnotations.value=metadata}
 catch(e){if(request===graphRequest)store.report(e)}
}
watch(()=>store.workspace?.id,()=>{graphRequest++;edges.value=[];graphAnnotations.value={revision:'',nodes:{},relations:{}};inspectedGraphEdge.value=undefined;inspectedGraphNode.value=undefined;graphPair.value=undefined;graphLinking.value=false;graphRelationsVisible.value=false})

let graphReturn:{tab:SidebarTab;page:Page}={tab:'navigation',page:'notes'}

function openGraph(){if(!graphOpen.value)graphReturn={tab:sidebarTab.value,page:page.value};graphInitialized.value=true;graphOpen.value=true;sidebarTab.value='graph';page.value='graph'}

function closeGraph(){graphOpen.value=false;sidebarTab.value=graphReturn.tab;page.value=graphReturn.page}

async function openGraphNote(id:string){const note=store.data.notes.find(n=>n.id===id);if(note){closeGraph();await openNote(note)}}

const pending = computed(() => store.data.tasks.filter(t => !t.done && !t.candidate).length)

type AttachmentView={attachment:Attachment;preview?:AttachmentPreview;loading:boolean;error:string;workspaceId:string}
const attachmentMain=ref<AttachmentView>(),attachmentModal=ref<AttachmentView>(),attachmentDownloading=ref(false),imageAttachment=ref<Attachment>()
let attachmentMainToken=0,attachmentModalToken=0
watch(()=>store.workspace?.id,()=>{attachmentMainToken++;attachmentModalToken++;attachmentMain.value=undefined;attachmentModal.value=undefined;imageAttachment.value=undefined})
watch(()=>store.active?.id,()=>metadataVisible.value=false)
const noteSearchDocument=computed(()=>editorDocument(store.content,store.active?.title,metadataVisible.value))
async function downloadAttachment(a:Attachment,workspaceId=store.workspace?.id){
 if(attachmentDownloading.value||!workspaceId||workspaceId!==store.workspace?.id)return
 if(a.remoteOnly&&a.remoteUrl){if(/^https?:\/\//i.test(a.remoteUrl))window.runtime?.BrowserOpenURL(a.remoteUrl);return}
 attachmentDownloading.value=true
 try{await api.SaveAttachment(a.id,a.name)}catch(e){notify(String(e))}finally{attachmentDownloading.value=false}
}
async function previewAttachment(a:Attachment,target:'main'|'modal'){
 const id=store.workspace?.id;if(!id)return
 if(/^image\/(png|jpeg|webp|gif|bmp|avif)(?:;|$)/.test(a.mime)){await run(async()=>{const data=await api.AttachmentData(a.id);if(store.workspace?.id!==id)return;showImage(data);imageAttachment.value=a});return}
 const token=target==='main'?++attachmentMainToken:++attachmentModalToken
 const state:AttachmentView={attachment:a,loading:true,error:'',workspaceId:id}
 if(target==='main')attachmentMain.value=state;else attachmentModal.value=state
 const current=()=>store.workspace?.id===id&&(target==='main'?attachmentMainToken===token:attachmentModalToken===token)
 try{const preview=await api.PreviewAttachment(id,a.id);if(!current())return;const done={...state,preview,loading:false};if(target==='main')attachmentMain.value=done;else attachmentModal.value=done}
 catch(e){if(!current())return;const failed={...state,loading:false,error:String(e)};if(target==='main')attachmentMain.value=failed;else attachmentModal.value=failed}
}
function closeAttachmentModal(){attachmentModalToken++;attachmentModal.value=undefined}

async function beforeAttachmentDelete(){await store.save();if(store.dirty||store.conflict)throw Error('当前笔记尚未保存，请先处理保存冲突')}
async function attachmentsDeleted(ids:string[]){if(imageAttachment.value&&ids.includes(imageAttachment.value.id)){image.value='';hiddenImage.value='';imageAttachment.value=undefined};if(attachmentMain.value&&ids.includes(attachmentMain.value.attachment.id))attachmentMain.value=undefined;if(attachmentModal.value&&ids.includes(attachmentModal.value.attachment.id))attachmentModal.value=undefined;await store.refresh()}

const outline = computed(() => { let fence = false; return store.content.split('\n').flatMap((text, index) => { if (/^\s*(```|~~~)/.test(text)) { fence = !fence; return [] } const match = /^(#{1,6})\s+(.+)$/.exec(text); return !fence && match ? [{ text: match[2], level: match[1]!.length, line: index }] : [] }) })

const wordCount = computed(() => store.content.replace(/^---[\s\S]*?---/, '').replace(/\s/g, '').length)

const related = computed(() => { const ids = new Set(edges.value.filter(e => e.source === store.active?.id || e.target === store.active?.id).flatMap(e => [e.source, e.target])); return store.data.notes.filter(n => n.id !== store.active?.id && ids.has(n.id)).slice(0, 4) })

const theme = computed(() => store.data.settings.theme === 'dark' ? darkTheme : null)

watch(() => store.data.settings.theme, value => document.documentElement.dataset.theme = value, { immediate: true })

watch(() => store.data.settings.picGoUrl, value => picgo.value = value)
const attachmentOptions=ref({picGoEnabled:true,keepOriginal:true,deduplicate:true})
watch(()=>store.data.settings.attachments,value=>{attachmentOptions.value={picGoEnabled:true,keepOriginal:true,deduplicate:true,...value}},{immediate:true})
async function saveAttachmentOptions(){await run(async()=>{await api.SetAttachmentPreferences(picgo.value,attachmentOptions.value);await store.refresh();notify('附件设置已保存')})}
async function restoreNoteAutomation(target:EntryTarget){contextMenu.value=undefined;await withEntry(target,async()=>{const note=store.data.notes.find(n=>n.path===target.path);if(!note)return;await api.SetNoteAutomationIgnored(target.workspaceId,note.id,note.hash,false);await store.refresh();notify('已恢复待办识别与自动分类，重新打开或编辑笔记后生效')})}


watch([page,()=>store.workspace?.id,()=>store.active?.id,()=>graphOpen.value?JSON.stringify(store.data.notes.map(n=>[n.id,n.hash,n.path])):''],()=>void refreshGraph())

function notify(text: string) { toast.value = text; clearTimeout(noticeTimer); noticeTimer = setTimeout(() => toast.value = '', 4000) }

async function run(fn: () => Promise<unknown>) { busy.value = true; try { await fn() } catch(e) { store.report(e) } finally { busy.value = false } }

type NoteTab={key:string;noteId?:string}
let blankTabSequence=0
const blankTab=():NoteTab=>({key:'blank:'+ ++blankTabSequence})
let noteOpening=false,openingNewNoteTab=false
const initialTab=blankTab(),noteTabs=ref<NoteTab[]>([initialTab]),activeNoteTab=ref(initialTab.key),noteTabBusy=ref(false)
const displayedNoteTabs=computed(()=>noteTabs.value.map(tab=>{const note=store.data.notes.find(n=>n.id===tab.noteId)||(store.active?.id===tab.noteId?store.active:undefined);return {key:tab.key,title:note?.path.split('/').pop()?.replace(/\.md$/i,'')||'新标签页',path:note?.path}}))
watch(()=>store.active,note=>{
 if(!note||store.switching)return
 const existing=noteTabs.value.find(tab=>tab.noteId===note.id)
 if(existing){activeNoteTab.value=existing.key;return}
 const current=noteTabs.value.find(tab=>tab.key===activeNoteTab.value),tab:NoteTab={key:'note:'+note.id,noteId:note.id}
 if(current&&(!openingNewNoteTab||!current.noteId))noteTabs.value.splice(noteTabs.value.indexOf(current),1,tab);else noteTabs.value.push(tab)
 activeNoteTab.value=tab.key
},{flush:'sync'})
watch(()=>store.workspace?.id,()=>{const tab=blankTab();noteTabs.value=[tab];activeNoteTab.value=tab.key;noteTabBusy.value=false},{flush:'sync'})
watch(()=>store.data.notes,notes=>{
 if(store.switching)return
 const ids=new Set(notes.map(n=>n.id));noteTabs.value=noteTabs.value.filter(tab=>!tab.noteId||ids.has(tab.noteId)||store.active?.id===tab.noteId)
 if(!noteTabs.value.length){const tab=blankTab();noteTabs.value=[tab];activeNoteTab.value=tab.key}
 if(!noteTabs.value.some(tab=>tab.key===activeNoteTab.value)){const next=noteTabs.value.at(-1)!;activeNoteTab.value=next.key;if(next.noteId){const note=notes.find(n=>n.id===next.noteId);if(note)void openNote(note)}}
})
function showNoteTabPage(){closeGraph();if(['trash','attachments','tasks'].includes(sidebarTab.value))selectSidebarTab('navigation');page.value='notes'}
async function blankNoteTab(create:boolean,key?:string){
 startupRestoreCancelled=true
 const workspace=store.workspace?.id;await store.save()
 if(store.dirty||store.switching||workspace!==store.workspace?.id)return false
 if(create){const tab=blankTab();noteTabs.value.push(tab);key=tab.key}
 activeNoteTab.value=key||noteTabs.value[0]!.key;store.active=null;store.content='';store.conflict=false;selectedEntry.value='';line.value=undefined;closeFind();showNoteTabPage()
 if(workspace)clearNoteSession(actualDataDirectory.value,workspace)
 return true
}
async function selectNoteTab(key:string){
 if(noteTabBusy.value||store.switching)return false
 const tab=noteTabs.value.find(tab=>tab.key===key);if(!tab)return false
 noteTabBusy.value=true
 try{if(tab.noteId){const note=store.data.notes.find(n=>n.id===tab.noteId);if(!note)return false;const opened=await openNote(note);if(opened)showNoteTabPage();return opened}return await blankNoteTab(false,key)}finally{noteTabBusy.value=false}
}
async function newNoteTab(){if(noteTabBusy.value||store.switching)return;noteTabBusy.value=true;try{await blankNoteTab(true)}finally{noteTabBusy.value=false}}
async function closeNoteTab(key=activeNoteTab.value){
 startupRestoreCancelled=true
 if(noteTabBusy.value||store.switching)return
 const index=noteTabs.value.findIndex(tab=>tab.key===key);if(index<0)return
 if(key===activeNoteTab.value){const next=noteTabs.value[index+1]||noteTabs.value[index-1];if(next){if(!await selectNoteTab(next.key))return}else{noteTabBusy.value=true;try{if(!await blankNoteTab(true))return}finally{noteTabBusy.value=false}}}
 noteTabs.value=noteTabs.value.filter(tab=>tab.key!==key)
}
async function closeNoteTabs(mode:'all'|'others'|'right',key:string){
 if(noteTabBusy.value||store.switching||busy.value)return
 const index=noteTabs.value.findIndex(tab=>tab.key===key);if(index<0)return
 const removed=new Set(noteTabs.value.filter((tab,i)=>mode==='all'||mode==='others'&&tab.key!==key||mode==='right'&&i>index).map(tab=>tab.key))
 if(!removed.size)return
 const workspace=store.workspace?.id
 noteTabBusy.value=true
 try{
  await store.save()
  if(store.dirty||store.switching||workspace!==store.workspace?.id)return
  if(removed.has(activeNoteTab.value)){
   const remaining=noteTabs.value.filter(tab=>!removed.has(tab.key)),next=remaining.find(tab=>tab.key===key)||remaining.at(-1)
   if(next?.noteId){const note=store.data.notes.find(note=>note.id===next.noteId);if(!note||!await openNote(note))return;showNoteTabPage()}
   else if(!await blankNoteTab(!next,next?.key))return
  }
  noteTabs.value=noteTabs.value.filter(tab=>!removed.has(tab.key))
 }finally{noteTabBusy.value=false}
}
async function stepNoteTab(delta:number){const i=noteTabs.value.findIndex(tab=>tab.key===activeNoteTab.value),n=noteTabs.value.length;if(n)await selectNoteTab(noteTabs.value[(i+delta+n)%n]!.key)}

type NoteLocation={id:string;line?:number}

const backHistory=ref<NoteLocation[]>([]),forwardHistory=ref<NoteLocation[]>([]),navigatingHistory=ref(false)

async function openNote(note: Note, targetLine?: number, record=true,newTab=false) {
 if(noteOpening||store.switching)return false
 noteOpening=true;openingNewNoteTab=newTab
 try {

 startupRestoreCancelled=true

 const previous=store.active?{id:store.active.id,line:line.value}:undefined

 if(!await store.select(note)) return false

 if(record && previous && previous.id!==note.id) {backHistory.value=[...backHistory.value,previous].slice(-100);forwardHistory.value=[]}

 if(folder.value && parentDirectory(note.path)!==currentDirectory.value) enterFolder(parentDirectory(note.path))

 treeTarget.value=entryTarget(note.path,false)

 selectedEntry.value='file:'+note.id;page.value='notes'

 if(!sourceVisible.value && !previewVisible.value) setMode(mode.value)

 await nextTick();line.value=undefined;await nextTick();line.value=targetLine;if(targetLine!==undefined){await nextTick();previewRef.value?.scrollToLine(targetLine)}

 return true
 } finally {noteOpening=false;openingNewNoteTab=false}
}

async function restoreLastOpenedNote(workspaceId?:string){
 if(appDisposed||startupRestoreCancelled||store.active||!workspaceId||store.workspace?.id!==workspaceId)return
 const saved=readNoteSession(actualDataDirectory.value,workspaceId)
 if(!saved)return
 const note=store.data.notes.find(note=>note.id===saved.id)||store.data.notes.find(note=>note.path===saved.path)
 if(!note){clearNoteSession(actualDataDirectory.value,workspaceId);return}
 await openNote(note,undefined,false)
}

async function browseHistory(direction:'back'|'forward') {

 if(busy.value || navigatingHistory.value || modal.value || archiveOpen.value || image.value) return

 const stack=direction==='back'?backHistory:forwardHistory,opposite=direction==='back'?forwardHistory:backHistory

 let index=stack.value.length-1

 while(index>=0 && !store.data.notes.some(note=>note.id===stack.value[index]!.id)) index--

 if(index<0) {stack.value=[];return}

 const target=stack.value[index]!,note=store.data.notes.find(note=>note.id===target.id)!

 const current=store.active?{id:store.active.id,line:line.value}:undefined

 navigatingHistory.value=true

 try {

  if(await openNote(note,target.line,false)) {

   stack.value=stack.value.slice(0,index)

   if(current) opposite.value=[...opposite.value,current].slice(-100)

  }

 } catch(error) {store.report(error)} finally {navigatingHistory.value=false}

}

function mouseHistory(event:MouseEvent) {

 if(event.button!==3 && event.button!==4) return

 event.preventDefault();event.stopPropagation()

 if(event.type==='mousedown') void browseHistory(event.button===3?'back':'forward')

}

watch(()=>store.autoArchiveResult, result=>{if(result&&store.workspace) {lastArchive.value={workspaceId:store.workspace.id,result};if(store.active?.id===result.note.id){enterFolder(parentDirectory(result.note.path));selectedEntry.value='file:'+result.note.id}notify('已自动归档至 '+result.note.path)}})

async function openArchive() {

 await store.save();if(store.dirty || !store.active) return

 archiveMinimized.value=false;archiveOpen.value=true

}

async function archiveApplied(result:ArchiveResult) {

 if(!store.workspace) return

 lastArchive.value={workspaceId:store.workspace.id,result}

 await showArchiveResult(result)

 notify(`整理完成，更新 ${result.updatedLinks} 处链接；可通过工具栏撤销`)

}

async function archiveBatchApplied(results:ArchiveResult[]) {
 if(!store.workspace || !results.length) return
 const activeResult=[...results].reverse().find(result=>result.note.id===store.active?.id)
 if(activeResult && !store.dirty){store.active=activeResult.note;store.content=activeResult.note.content;lastArchive.value={workspaceId:store.workspace.id,result:activeResult};enterFolder(parentDirectory(activeResult.note.path));treeTarget.value=entryTarget(activeResult.note.path,false);selectedEntry.value='file:'+activeResult.note.id}
 await store.refresh()
 const links=results.reduce((sum,result)=>sum+result.updatedLinks,0)
 notify(`历史笔记整理完成：${results.length} 篇，更新 ${links} 处链接；各篇可在整理列表中单独撤销`)
}

async function archiveBatchUndone(note:Note) {
 if(store.active?.id===note.id && !store.dirty){store.active=note;store.content=note.content;enterFolder(parentDirectory(note.path));treeTarget.value=entryTarget(note.path,false);selectedEntry.value='file:'+note.id;lastArchive.value=undefined}
 await store.refresh()
 notify('已撤销该篇历史笔记整理')
}

async function showArchiveResult(result:ArchiveResult) {

 store.active=result.note;store.content=result.note.content

 await store.refresh();await openTreeNote(result.note)

}

async function undoLastArchive() {

 const saved=lastArchive.value;if(!saved || busy.value) return

 await run(async()=>{

  await store.save();if(store.dirty || !store.active) return

  const result=await api.UndoArchive(saved.workspaceId,saved.result.operationId,store.active.hash)

  lastArchive.value=undefined;store.suppressAutomaticArchive(result.note);await showArchiveResult(result);notify('上次整理已撤销')

 })

}

async function addWorkspace() { if(busy.value)return; await run(async()=>{await store.chooseWorkspace()}) }

async function createNote() { if (busy.value) return; await store.save(); if (store.dirty) return; await run(async () => { const n = await api.CreateNote(noteTitle.value || '未命名笔记', noteDir.value.trim() || 'Inbox'); modal.value = null; await store.refresh(); await openTreeNote(n); setMode('edit'); noteTitle.value = '' }) }

function newNote() { if (!store.workspace) { void addWorkspace(); return } noteDir.value = folder.value === '.' ? '.' : folder.value || 'Inbox'; modal.value = 'note' }

async function moveNote() {

 await store.save(); if (!store.active || store.dirty) return

 await run(async () => { const result=await api.MoveNote(store.active!.id,store.active!.hash,movePath.value); store.active=null; await store.refresh(); await openTreeNote(result.note); modal.value=null; notify(`笔记已移动，更新 ${result.updatedLinks} 处链接，原文副本已保留`) })

}

async function switchWorkspace(id: string) { if(busy.value)return; await run(async()=>{await store.switchWorkspace(id)}) }

async function navigateWorkspace(id:string,path:string) { if(busy.value) return; if(store.workspace?.id!==id) await switchWorkspace(id); if(store.workspace?.id===id) enterFolder(path) }

async function navigateWorkspaceNote(id:string,note:Pick<Note,'path'> & {id?:string},matchLine?:number,newTab=false) { if(busy.value) return; if(store.workspace?.id!==id) await switchWorkspace(id); if(store.workspace?.id!==id) return; const current=store.data.notes.find(item=>item.id===note.id || item.path===note.path); if(current) await openTreeNote(current,matchLine,newTab) }

type EntryTarget = { workspaceId:string; path:string; isDirectory:boolean }

const treeTarget=ref<EntryTarget>(), operationTarget=ref<EntryTarget>(),copiedEntry=ref<EntryTarget>()

const entryName=ref(''), contextMenu=ref<{x:number;y:number;target:EntryTarget;ready:boolean}>(), shortcutCapture=ref<CommandId>(), shortcutError=ref(''), commandQuery=ref('')

watch([settingsOpen,settingsTab],()=>{if(!settingsOpen.value || settingsTab.value!=='shortcuts') {shortcutCapture.value=undefined; shortcutError.value=''}})

type DialogKind=NonNullable<typeof modal.value> | 'taskDetails'

type TaskDialog={taskId:string;dueAt:string;priority:string}

type SavedDialog={id:number;type:DialogKind;workspaceId:string;noteTitle:string;noteDir:string;rename:string;movePath:string;entryName:string;query:string;target?:EntryTarget;task?:TaskDialog}

const hiddenDialogs=ref<SavedDialog[]>([])

let dialogSequence=0

const dialogNames:Record<DialogKind,string>={note:'新建笔记',delete:'删除笔记',conflict:'外部修改冲突',remove:'移除工作区注册',rename:'重命名工作区',move:'移动笔记',entryRename:'重命名',entryDelete:'移入回收目录',folder:'新建文件夹',palette:'命令面板',quickOpen:'快速打开笔记',taskDetails:'任务详情'}

function minimizeDialog(task?:TaskDialog) {

 const type=task?'taskDetails':modal.value; if(!type) return

 hiddenDialogs.value.push({id:++dialogSequence,type,workspaceId:store.workspace?.id || '',noteTitle:noteTitle.value,noteDir:noteDir.value,rename:rename.value,movePath:movePath.value,entryName:entryName.value,query:commandQuery.value,target:operationTarget.value?{...operationTarget.value}:undefined,task})

 if(!task) modal.value=null

}

async function restoreDialog(id:number) {

 const saved=hiddenDialogs.value.find(item=>item.id===id); if(!saved || busy.value) return

 await run(async()=>{

  await store.save(); if(store.dirty) throw new Error('请先处理未保存的修改。')

  if(modal.value) minimizeDialog()

  if(saved.workspaceId && saved.workspaceId!==store.workspace?.id) {

   if(!store.data.settings.workspaces.some(w=>w.id===saved.workspaceId)) throw new Error('该窗口的工作区已移除。')

   await store.switchWorkspace(saved.workspaceId)

   if(store.workspace?.id!==saved.workspaceId) throw new Error('无法切换到窗口所属工作区。')

  }

  noteTitle.value=saved.noteTitle;noteDir.value=saved.noteDir;rename.value=saved.rename;movePath.value=saved.movePath;entryName.value=saved.entryName;commandQuery.value=saved.query;operationTarget.value=saved.target

  if(saved.type==='taskDetails' && saved.task) {page.value='tasks';await nextTick();todo.value?.restoreDetails(saved.task)} else modal.value=saved.type as NonNullable<typeof modal.value>

  hiddenDialogs.value=hiddenDialogs.value.filter(item=>item.id!==id);layoutOpen.value=false

 })

}

const commandIndex=ref(0)

watch(commandQuery,()=>commandIndex.value=0)

const paletteCommands=computed(()=>commands.filter(command=>command.label.toLowerCase().includes(commandQuery.value.toLowerCase())))

const quickNotes=computed(()=>store.data.notes.filter(note=>(note.title+' '+note.path).toLowerCase().includes(commandQuery.value.toLowerCase())).slice(0,60))

const hotkey=(id:CommandId)=>formatShortcut(binding(id,ui.value.shortcuts))

const tabCommands:Record<SidebarTab,CommandId>={navigation:'navigation',tasks:'tasks',search:'globalSearch',graph:'graph',attachments:'attachments',bookmarks:'bookmarks',trash:'trash'}

const entryTarget=(path:string,isDirectory:boolean):EntryTarget=>({workspaceId:store.workspace!.id,path,isDirectory})

function focusedEntry() { return treeTarget.value || (store.active ? entryTarget(store.active.path,false) : store.workspace ? entryTarget(currentDirectory.value,true) : undefined) }

function focusEntry(workspaceId:string,path:string,isDirectory:boolean) { treeTarget.value={workspaceId,path,isDirectory} }

async function openContextNote(target:EntryTarget,newTab=false){
 contextMenu.value=undefined
 await navigateWorkspaceNote(target.workspaceId,{path:target.path},undefined,newTab)
}

function showContext(event:MouseEvent,workspaceId:string,path:string,isDirectory:boolean) {

 const target={workspaceId,path,isDirectory}; treeTarget.value=target

 ;(event.target as HTMLElement).closest<HTMLElement>('.workspace-tree-row,.workspace-root,.note-card,.sidebar-note-result,.directory-card')?.focus()


 const position=contextMenuPosition(event.clientX,event.clientY,274,360)
 contextMenu.value={...position,target,ready:false}
 const opened=contextMenu.value
 void nextTick(()=>{const menu=document.querySelector<HTMLElement>('.entry-context-menu');if(menu&&contextMenu.value===opened){const rect=menu.getBoundingClientRect();Object.assign(opened,contextMenuPosition(event.clientX,event.clientY,rect.width,rect.height),{ready:true});void nextTick(()=>{if(contextMenu.value===opened)menu.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus({preventScroll:true})})}})

}

async function withEntry(target:EntryTarget,action:()=>Promise<void>) {

 if(busy.value) return

 await run(async()=>{

  await store.save(); if(store.dirty) throw new Error('请先处理未保存的修改。')

  if(store.workspace?.id!==target.workspaceId) await store.switchWorkspace(target.workspaceId)

  if(store.workspace?.id!==target.workspaceId) throw new Error('无法切换到目标工作区。')

  await action()

 })

}

function remapOrder(workspaceId:string,source:string,target?:string) {

 const under=(path:string)=>path===source || path.startsWith(source+'/')

 const map=(path:string)=>under(path)?target!+path.slice(source.length):path

 const orders:Record<string,string[]>={}

 for(const [parent,paths] of Object.entries(ui.value.entryOrder[workspaceId] || {})) {

  if(!target && under(parent)) continue

  orders[target?map(parent):parent]=paths.filter(path=>target || !under(path)).map(path=>target?map(path):path)

 }

 if(target && parentDirectory(source)!==parentDirectory(target)) {

  const oldParent=parentDirectory(source),newParent=parentDirectory(target)

  if(orders[oldParent]) orders[oldParent]=orders[oldParent]!.filter(path=>path!==target)

  if(orders[newParent] && !orders[newParent]!.includes(target)) orders[newParent]!.push(target)

 }

 ui.value.entryOrder={...ui.value.entryOrder,[workspaceId]:orders}; persistUI()

}

async function applyEntryMove(source:EntryTarget,target:string) {

 const activeId=store.active?.id

 if(source.isDirectory) { const result=await api.MoveDirectory(source.path,target); notify(`文件夹已移动，更新 ${result.updatedLinks} 处链接`) }

 else {

  const note=store.data.notes.find(note=>note.path===source.path); if(!note) throw new Error('源文件已变化，请刷新后重试。')

  const result=await api.MoveNote(note.id,note.hash,target); notify(`笔记已移动，更新 ${result.updatedLinks} 处链接`)

 }

 remapOrder(source.workspaceId,source.path,target)

 store.active=null; await store.refresh()

 const active=store.data.notes.find(note=>note.id===activeId); if(active) await store.select(active)

 if(source.isDirectory) enterFolder(target)

 else { const moved=store.data.notes.find(note=>note.path===target); if(moved) await openTreeNote(moved) }

 treeTarget.value={...source,path:target}


}

async function moveDraggedEntry(workspaceId:string,path:string,isDirectory:boolean,directory:string) {

 const target=(directory?directory+'/':'')+path.split('/').pop()!

 if(target===path) return

 await withEntry({workspaceId,path,isDirectory},()=>applyEntryMove({workspaceId,path,isDirectory},target))

}

function requestRename(target=focusedEntry()) {

 contextMenu.value=undefined; if(!target?.path) return

 store.error=''; operationTarget.value={...target}; entryName.value=target.path.split('/').pop()!; modal.value='entryRename'

}

function requestDelete(target=focusedEntry()) {

 contextMenu.value=undefined; if(!target?.path) return

 store.error=''; operationTarget.value={...target}; modal.value='entryDelete'

}

function requestFolder(target=focusedEntry()) {

 contextMenu.value=undefined; if(!target && !store.workspace) return

 store.error=''; operationTarget.value=target?{...target,path:target.isDirectory?target.path:parentDirectory(target.path),isDirectory:true}:entryTarget(currentDirectory.value,true)

 entryName.value=''; modal.value='folder'

}

async function renameEntry() {

 const source=operationTarget.value; if(!source) return

 const name=entryName.value.trim()

 if(!name || name==='.' || name==='..' || /[\\/:*?"<>|]/.test(name)) { store.error='请输入有效的名称，不要包含路径分隔符。'; return }

 const filename=!source.isDirectory && !name.toLowerCase().endsWith('.md')?name+'.md':name

 const target=(parentDirectory(source.path)?parentDirectory(source.path)+'/':'')+filename

 if(source.path===target) { modal.value=null; return }

 await withEntry(source,async()=>{ await applyEntryMove(source,target); modal.value=null })

}

async function createFolder() {

 const target=operationTarget.value; if(!target) return

 const name=entryName.value.trim(); if(!name || name==='.' || name==='..') {store.error='请输入文件夹名称。'; return}

 const parts=((target.path?target.path+'/':'')+name).replace(/\\/g,'/').split('/')

 if(parts.includes('..')) {store.error='文件夹路径不能包含 ..。'; return}

 const path=parts.filter(part=>part && part!=='.').join('/')

 await withEntry(target,async()=>{await api.CreateDirectory(path); await store.refresh(); enterFolder(path); treeTarget.value={...target,path}; modal.value=null; notify('文件夹已创建')})

}

async function deleteEntry() {

 const target=operationTarget.value; if(!target?.path) return

 await withEntry(target,async()=>{

  if(target.isDirectory) { const result=await api.DeleteDirectory(target.path); notify(`已移入回收目录：${result.recoveryPath}`) }

  else { const note=store.data.notes.find(note=>note.path===target.path); if(!note) throw new Error('文件不存在，请刷新。'); await api.DeleteNote(note.id,note.hash); notify('笔记已移入应用数据目录的 trash，附件保留') }

  const under=(path:string)=>path===target.path || target.isDirectory && path.startsWith(target.path+'/')

  if(store.active && under(store.active.path)) store.active=null

  if(under(currentDirectory.value)) enterFolder(parentDirectory(target.path))

  remapOrder(target.workspaceId,target.path)


  treeTarget.value=undefined; modal.value=null; await store.refresh()

 })

}

function copyEntry(target=focusedEntry()) {
 contextMenu.value=undefined;if(!target?.path)return
 copiedEntry.value={...target};notify('已复制，请在目标目录粘贴')
}
async function pasteEntry(target=focusedEntry()) {
 contextMenu.value=undefined
 const source=copiedEntry.value;if(!source||!target)return
 if(source.workspaceId!==target.workspaceId){store.error='请在同一工作区内粘贴';return}
 const directory=target.isDirectory?target.path:parentDirectory(target.path)
 await withEntry(target,async()=>{
  const result=await api.PasteEntry(source.path,directory)
  await store.refresh()
  await nextTick();await treeInstances.get(target.workspaceId)?.reveal(result.path,{focus:false})
  notify(`已粘贴副本：${result.path.split('/').pop()}`)
 })
}

async function newNoteAt(target:EntryTarget) {

 contextMenu.value=undefined

 await withEntry(target,async()=>{noteDir.value=(target.isDirectory?target.path:parentDirectory(target.path)) || '.'; modal.value='note'})

}

async function copyEntryPath(absolute=false,target=focusedEntry()) {

 contextMenu.value=undefined; if(!target) return

 if(absolute) await withEntry(target,async()=>{await navigator.clipboard.writeText(await api.EntryLocation(target.path || '.')); notify('完整路径已复制')})

 else {await navigator.clipboard.writeText(target.path || '.'); notify('相对路径已复制')}

}

async function revealEntry(target=focusedEntry()) {

 contextMenu.value=undefined; if(target) await withEntry(target,()=>api.RevealEntry(target.path || '.'))

}

function menuKeys(event:KeyboardEvent) {

 if(event.key==='Escape') {event.preventDefault(); contextMenu.value=undefined; return}

 if(!['ArrowUp','ArrowDown','Home','End'].includes(event.key)) return

 event.preventDefault()

 const buttons=Array.from((event.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('button:not(:disabled)'))

 const index=buttons.indexOf(document.activeElement as HTMLButtonElement),delta=event.key==='ArrowUp'?-1:1

 buttons[event.key==='Home'?0:event.key==='End'?buttons.length-1:(index+delta+buttons.length)%buttons.length]?.focus()

}

function closeContext(event?:Event) { const element=event?.target instanceof Element?event.target:undefined; if(event && !element?.closest('.shortcut-key')) {shortcutCapture.value=undefined; shortcutError.value=''}; if(!element?.closest('.entry-context-menu')) contextMenu.value=undefined; if(!element?.closest('.layout-tools')) layoutOpen.value=false }

function startCapture(id:CommandId) { shortcutCapture.value=id; shortcutError.value='' }

function resetShortcut(id?:CommandId) {

 const next={...ui.value.shortcuts}; if(id) {delete next[id]; const issue=shortcutIssue(id,binding(id),next); if(issue) {shortcutError.value=issue; return}}

 ui.value.shortcuts=id?next:{}; shortcutCapture.value=undefined; shortcutError.value=''; persistUI()

}

function captureShortcut(event:KeyboardEvent) {

 event.preventDefault(); event.stopPropagation()

 if(event.key==='Escape') {shortcutCapture.value=undefined; shortcutError.value=''; return}

 const id=shortcutCapture.value!; const key=event.key==='Backspace'?'':keyStroke(event)

 if(event.key!=='Backspace' && !key) return

 const issue=shortcutIssue(id,key,ui.value.shortcuts || {}); if(issue) {shortcutError.value=issue; return}

 ui.value.shortcuts={...ui.value.shortcuts,[id]:key}; persistUI(); shortcutCapture.value=undefined; shortcutError.value=''

}

async function executeCommand(id:CommandId) {

 contextMenu.value=undefined

 if(id==='back' || id==='forward') {await browseHistory(id);return}

 if(id==='addBookmark'){await addBookmark();return}
 if(id==='toggleNoteBookmark'){await toggleNoteBookmark();return}

 if(id==='bookmarks'||id==='trash'){selectSidebarTab(id);return}

 if(id==='syncNow'){if(store.workspace){await store.save();if(!store.dirty)try{await api.StartSync(store.workspace.id);notify('正在后台同步')}catch(e){notify(String(e))}}return}
 if(id==='syncSettings'){settingsTab.value='sync';settingsOpen.value=true;return}
 if(id==='llmPolish'){toggleAI();return}
 if(id==='llmSummary'){await prepareLLM('summary');return}
 if(id==='organize') {await openArchive();return}

 if(id==='palette' || id==='quickOpen') {commandQuery.value=''; commandIndex.value=0; modal.value=id; return}

 if(id==='newNote') {const target=focusedEntry(); if(target && document.activeElement?.closest('.workspace-tree-row,.workspace-root,.note-card,.sidebar-note-result,.directory-card')) await newNoteAt(target); else newNote(); return}

 if(id==='newFolder') {requestFolder(); return}

 if(id==='save') {await store.save(); return}

 if(id==='openWorkspace') {await addWorkspace(); return}

 if(id==='navigation'){selectSidebarTab('navigation');return}

 if(id==='globalReplace'){if(globalSearchKind.value==='semantic')globalSearchKind.value='text';selectSidebarTab('search');await nextTick();workspaceSearch.value?.openReplace();document.querySelector<HTMLInputElement>('[aria-label="全局查找内容"]')?.focus();return}

 if(id==='search' || id==='globalSearch'||id==='searchTab') {selectSidebarTab('search'); await nextTick(); document.querySelector<HTMLInputElement>('.search-query input, [aria-label="全局查找内容"]')?.focus(); return}

 if(id==='replace'){findReplaceOpen.value=true;await openFind();return}

 if(id==='navigationFind'){await openNavigationSearch();return}
 if(id==='find'){if(document.activeElement?.closest('#sidebar-panel-navigation')||document.activeElement?.id==='sidebar-tab-navigation')await openNavigationSearch();else await openFind();return}

 if(id==='closeNote'){await closeNoteTab();return}
 if(id==='newTab'){await newNoteTab();return}
 if(id==='nextNoteTab'||id==='previousNoteTab'){await stepNoteTab(id==='nextNoteTab'?1:-1);return}

 if(id==='attach') {if(store.active) fileInput.value?.click(); return}

 if(id==='export') {if(store.workspace) await exportWorkspace(false); return}

 if(id==='backup') {if(store.workspace) await exportWorkspace(true); return}

 if(id==='import') {await importWorkspace(); return}

 if(id==='theme') {await setTheme(); return}

 if(id==='tasks' || id==='graph' || id==='attachments') {selectSidebarTab(id); return}

 if(id==='settings') {settingsOpen.value=true; return}

 if(id==='sidebar') {collapsed.value=!collapsed.value; return}

 if(id==='preview') {setMode(mode.value==='preview'?'edit':'preview'); return}

 if(id==='split') {setMode('split'); return}

 if(id==='refresh') {await store.save(); if(!store.dirty) await store.refresh(); return}

 if(id==='rename') {requestRename(); return}

 if(id==='delete') {requestDelete(); return}

 if(id==='copyEntry') {copyEntry();return}
 if(id==='pasteEntry') {await pasteEntry();return}

 if(id==='copyPath') {await copyEntryPath(); return}

 if(id==='reveal') {await revealEntry(); return}

 if(id==='contextMenu') {const target=focusedEntry(); if(target) {const rect=(document.activeElement as HTMLElement)?.getBoundingClientRect(); showContext({clientX:rect?.left || 20,clientY:rect?.bottom || 100,target:document.activeElement} as MouseEvent,target.workspaceId,target.path,target.isDirectory)}; return}

 if(store.active) { if(mode.value==='preview') setMode('edit'); enterFolder(parentDirectory(store.active.path)); selectedEntry.value='file:'+store.active.id; page.value='notes'; await nextTick(); await editor.value?.command(id) }

}

function paletteKeys(event:KeyboardEvent) {

 const count=modal.value==='palette'?paletteCommands.value.length:quickNotes.value.length

 if(['ArrowUp','ArrowDown'].includes(event.key)) {event.preventDefault(); commandIndex.value=count?(commandIndex.value+(event.key==='ArrowUp'?-1:1)+count)%count:0; void nextTick(()=>document.querySelector('.command-results .chosen')?.scrollIntoView({block:'nearest'})); return}

 if(event.key==='Enter') {event.preventDefault(); if(modal.value==='palette') {const command=paletteCommands.value[commandIndex.value]; if(command) void runPalette(command.id)} else {const note=quickNotes.value[commandIndex.value]; if(note) {modal.value=null; void openTreeNote(note)}}}

}

async function runPalette(id:CommandId) { modal.value=null; await nextTick(); await executeCommand(id) }

const globalSearchKind=ref('semantic'),workspaceSearch=ref<InstanceType<typeof WorkspaceTextSearch>>()

const searchMessage=ref(''),searchMode=ref('hybrid')

let searchSequence=0,searchDebounce:ReturnType<typeof setTimeout>|undefined

watch([query,()=>store.workspace?.id],()=>{searchSequence++;results.value=[];searched.value=false;searching.value=false;searchMessage.value='';clearTimeout(searchDebounce);if(query.value.trim())searchDebounce=setTimeout(()=>void search(),450)})

async function search(){

 clearTimeout(searchDebounce)

 const text=query.value.trim(),id=store.workspace?.id,seq=++searchSequence

 if(!text||globalSearchKind.value!=='semantic'){results.value=[];searched.value=false;return}

 searching.value=true;searchMessage.value='正在用内置模型分析笔记语义…'

 try{await store.save();if(seq!==searchSequence||store.dirty)return;const response=await api.SemanticSearch(text);if(seq!==searchSequence||store.workspace?.id!==id)return;results.value=response.results||[];searchMode.value=response.mode;searchMessage.value=response.message;searched.value=true}catch(e){if(seq===searchSequence){searchMessage.value='搜索失败：'+String(e);results.value=[];searched.value=false}}finally{if(seq===searchSequence)searching.value=false}

}

watch(globalSearchKind,()=>{searchSequence++;clearTimeout(searchDebounce);if(globalSearchKind.value==='semantic'&&query.value.trim())void search()})

let attachmentUploadQueue=Promise.resolve()
function upload(file:File,paste=false){
 const noteId=store.active?.id,workspaceId=store.workspace?.id
 const queued=attachmentUploadQueue.then(async()=>{
  if(store.active?.id!==noteId||store.workspace?.id!==workspaceId){notify('笔记已切换，请重新插入附件');return}
  await importNoteAttachment(file,paste)
 })
 attachmentUploadQueue=queued.catch(error=>store.report(error))
 return queued
}
async function importNoteAttachment(file: File, paste = false) {

 if (!store.active) { store.error = '请先打开一篇笔记，再插入附件。'; return }

 if (file.size > 100 * 1024 * 1024) { store.error = '附件不能超过 100 MB'; return }

 const noteId = store.active.id

 await run(async () => {

  const base64 = await new Promise<string>((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result).split(',')[1]!); reader.onerror = reject; reader.readAsDataURL(file) })

  const a = await api.ImportAttachment(file.name || `截图-${Date.now()}.png`, base64, paste)

  if (store.active?.id !== noteId) { notify('附件已保存至附件库'); return }

  const url = a.remoteUrl && paste ? a.remoteUrl : `smartnote-asset://${a.id}`

  const label = a.name.replace(/[\[\]\\\r\n]/g, '-')

  const text = `\n${a.mime.startsWith('image/') ? '!' : ''}[${label}](${url})\n`

  if (editor.value) editor.value.insert(text); else store.edit(store.content + text)

  await store.save(); await store.refresh(); notify(paste && a.mime.startsWith('image/') && (store.data.settings.attachments?.picGoEnabled??true) ? (a.remoteUrl?'图片已上传，已插入图床链接':'PicGo 上传失败，已回退本地附件') : '附件已保存')

 })

}

async function fileChanged(e: Event) { for (const file of Array.from((e.target as HTMLInputElement).files ?? [])) await upload(file); (e.target as HTMLInputElement).value = '' }

async function openAttachment(a:Attachment){await previewAttachment(a,'modal')}

function showImage(src: string,attachmentId?:string) { imageAttachment.value=store.data.attachments.find(a=>attachmentId?a.id===attachmentId:a.remoteUrl===src);image.value = src }


async function followLink(target: string) {

 if (/^https?:\/\//i.test(target)) { window.runtime?.BrowserOpenURL(target); return }

 if (target.startsWith('smartnote-asset://')) { const a = store.data.attachments.find(a => a.id === target.slice(18)); if (a) await openAttachment(a); return }

 const wiki=target.startsWith('smartnote-note://')

 let noteTarget:string,heading:string|undefined

 try {

  const parts=wiki ? decodeURIComponent(target.slice(17)).split('#') : target.split('#').map(part=>decodeURIComponent(part))

  noteTarget=parts[0] || '';heading=parts[1]

 } catch { return }

 const normalize=(path:string)=> { const parts:string[]=[]; for(const part of path.split('/')) { if(part==='..')parts.pop();else if(part && part!=='.')parts.push(part) };return parts.join('/') }

 const relative=normalize((store.active?.path.split('/').slice(0,-1).join('/') || '')+'/'+noteTarget)

 const paths=wiki ? [noteTarget,noteTarget+'.md',relative,relative+'.md'] : [relative,relative+'.md',noteTarget,noteTarget+'.md']

 const note = !noteTarget ? store.active : paths.map(path=>store.data.notes.find(n=>n.path===path)).find(Boolean) || store.data.notes.find(n=>n.title===noteTarget)

 if (note) { const targetLine=heading ? note.content.split('\n').findIndex(l=>l.replace(/^#+\s*/, '').trim()===heading) : undefined; await openNote(note,targetLine!==undefined && targetLine>=0 ? targetLine : undefined) } else notify('未找到对应笔记')

}

async function exportWorkspace(native: boolean) { await store.save(); if (store.dirty) return; await run(async () => { const path = await api.Export(native); if (path) notify(`已导出至 ${path}`) }) }

function importWorkspace(){folderImportOpen.value=true}
async function importZipWorkspace() {if(busy.value)return;await run(async()=>{if(await store.importZip())notify('导入完成，已打开新工作区')})}

async function setTheme() { await run(async () => { await api.SetPreferences(store.data.settings.theme === 'dark' ? 'light' : 'dark', picgo.value); await store.refresh() }) }

function keydown(e:KeyboardEvent) {
 if(folderImportOpen.value)return
 if(e.target instanceof Element&&e.target.closest('.chat-model-menu')&&['Escape','ArrowDown','ArrowUp','Home','End','Enter'].includes(e.key))return
 if(store.switching){e.preventDefault();return}
 if(document.getElementById('workspace-menu') && ['Escape','ArrowDown','ArrowUp','Home','End'].includes(e.key))return

 if(e.target instanceof Element && e.target.closest('.graph-inline-editor'))return
 if(image.value && e.key==='Escape')return
 if(e.target instanceof Element && e.target.closest('.knowledge-graph') && (e.code==='Space'||e.key===' '))return
 if(attachmentModal.value || (e.target instanceof Element && e.target.closest('.trash-confirmation'))) return
 if(shortcutCapture.value) {captureShortcut(e); return}

 if(e.isComposing || e.defaultPrevented) return

 if(e.key==='Escape' && e.target instanceof Element && e.target.closest('.cm-editor')?.querySelector('.cm-admonition-completion'))return

 if(e.key==='Escape' && (e.target as HTMLElement)?.closest('.navigation-search')){e.preventDefault();e.stopPropagation();closeNavigationSearch();return}

 if(e.key==='Escape'&&llmOpen.value){llmOpen.value=false;e.preventDefault();return}
 if(e.key==='Escape') {if(graphOpen.value){e.preventDefault();closeGraph();return};if(findOpen.value){e.preventDefault();closeFind();return};contextMenu.value=undefined; image.value=''; return }

 if(modal.value || archiveOpen.value || image.value || busy.value || llmOpen.value) return

 const element=e.target as HTMLElement

 const inEditor=!!element.closest('.cm-content')

 const editable=!!element.closest('input,textarea,select,[contenteditable="true"]')

 const inExplorer=!editable && !!element.closest('.workspace-tree-row,.workspace-root,.note-card,.sidebar-note-result,.directory-card,.note-list,.directory-location')

 const stroke=keyStroke(e); if(!stroke) return

 const available=commands.filter(command=>command.scope==='app' || command.scope==='editor' && inEditor || command.scope==='explorer' && inExplorer)

 const command=available.find(command=>binding(command.id,ui.value.shortcuts)===stroke)

 if(command) {

  // Native clipboard events retain screenshot/file paste support in CodeMirror.

  if(inEditor && ['copy','cut','paste'].includes(command.id) && command.key===stroke) return

  e.preventDefault(); e.stopPropagation(); void executeCommand(command.id).catch(store.report); return

 }

 // Suppress the previous binding as well as the browser/CodeMirror default.

 if(available.some(command=>command.key===stroke && binding(command.id,ui.value.shortcuts)!==stroke)) {e.preventDefault(); e.stopPropagation()}

}

function floatingEscape(event:KeyboardEvent) {if(event.key==='Escape' && !event.defaultPrevented && !modal.value && settingsOpen.value && event.target instanceof Element && event.target.closest('.floating-window') && !event.target.closest('select,[role="combobox"]')) settingsOpen.value=false}

function beforeUnload(e: BeforeUnloadEvent) { if (store.dirty) { e.preventDefault(); e.returnValue = ''; void store.save() } }

let externalTimer:ReturnType<typeof setTimeout>|undefined,externalRunning=false,externalPending=false,appDisposed=false

function requestWorkspaceCheck(){

 externalPending=true;clearTimeout(externalTimer)

 externalTimer=setTimeout(async()=>{

  if(appDisposed)return

  if(busy.value||store.saving||externalRunning){requestWorkspaceCheck();return}

  externalPending=false;externalRunning=true

  try{await store.refresh()}finally{externalRunning=false;if(externalPending&&!appDisposed)requestWorkspaceCheck()}

 },200)

}

onMounted(async () => { warmCalendar(); unlisten = window.runtime?.EventsOn('workspace:check', requestWorkspaceCheck); await store.refresh(); const startupWorkspaceId=store.workspace?.id; if (desktop) { try { actualDataDirectory.value = await api.DataDirectory() } catch { /* Older preview bridge may omit this method. */ } } noteSessionReady.value=true; await restoreLastOpenedNote(startupWorkspaceId); document.addEventListener('mousedown',mouseHistory,true); document.addEventListener('mouseup',mouseHistory,true); document.addEventListener('auxclick',mouseHistory,true); document.addEventListener('keydown', keydown, true); document.addEventListener('keydown',floatingEscape); document.addEventListener('wheel',editorWheel,{capture:true,passive:false}); document.addEventListener('pointerdown',closeContext); window.addEventListener('blur',closeContext); window.addEventListener('beforeunload', beforeUnload) })

onBeforeUnmount(() => { appDisposed=true;clearTimeout(externalTimer);unlisten?.(); document.removeEventListener('mousedown',mouseHistory,true); document.removeEventListener('mouseup',mouseHistory,true); document.removeEventListener('auxclick',mouseHistory,true); document.removeEventListener('keydown', keydown, true); document.removeEventListener('keydown',floatingEscape); document.removeEventListener('wheel',editorWheel,true); document.removeEventListener('pointerdown',closeContext); window.removeEventListener('blur',closeContext); window.removeEventListener('beforeunload', beforeUnload); clearTimeout(timer); clearTimeout(noticeTimer);clearTimeout(searchDebounce) })

</script>

<template>

 <NConfigProvider :locale="zhCN" :date-locale="dateZhCN" :theme="theme" :theme-overrides="{ common: { primaryColor: '#7770c4', primaryColorHover: '#8982d2', primaryColorSuppl: '#7770c4', borderRadius: '7px', fontFamily: fontStack(ui.fontFamily), fontSize: ui.fontSize + 'px', fontSizeSmall: (ui.fontSize - 2) + 'px', fontSizeMedium: ui.fontSize + 'px', fontSizeLarge: (ui.fontSize + 2) + 'px' } }">

 <div class="app-shell" :inert="store.switching||undefined" :style="{'--sidebar-width':layout.sidebarWidth+'px','--list-width':layout.listWidth+'px','--inspector-width':layout.inspectorWidth+'px'}" :class="{ 'sidebar-collapsed': collapsed }">

   <nav class="main-nav sidebar-tabs sidebar-rail" role="tablist" aria-label="侧边栏页签" aria-orientation="vertical" @keydown="sidebarTabKey"><button v-for="item in sidebarTabs" :key="item.id" :id="'sidebar-tab-'+item.id" role="tab" :aria-label="item.label" :title="item.label+'（'+hotkey(tabCommands[item.id])+'）'" :aria-keyshortcuts="accessibleShortcut(binding(tabCommands[item.id],ui.shortcuts))" :aria-selected="sidebarTab===item.id" :aria-controls="'sidebar-panel-'+item.id" :tabindex="sidebarTab===item.id?0:-1" :class="{active:sidebarTab===item.id}" @click="selectSidebarTab(item.id)"><component :is="item.icon" :size="18"/><span class="sidebar-tab-label">{{item.label}}</span></button></nav>

  <aside class="sidebar">

   <div class="brand"><span class="brand-mark"><BookOpen :size="21" /></span><strong>SmartNote<span class="brand-dot">.</span></strong><span class="version">v{{ appVersion }}</span></div>

   <WorkspacePicker :workspaces="store.data.settings.workspaces" :current-id="store.workspace?.id" :disabled="busy||store.switching" @select="switchWorkspace" @create="addWorkspace" @rename="rename=store.workspace?.name||'';modal='rename'" @remove="modal='remove'"/>

   <div class="sidebar-tab-content">

    <section v-show="sidebarTab==='navigation'" id="sidebar-panel-navigation" class="sidebar-navigation" tabindex="0" role="tabpanel" aria-labelledby="sidebar-tab-navigation">

   <div class="sidebar-section"><span>工作区目录</span><div class="directory-tools"><button class="icon-button" aria-label="查询导航栏" :title="'按含义、文件名或内容查询（'+(hotkey('navigationFind')||hotkey('find'))+'）'" :aria-expanded="navigationSearchOpen" @click="openNavigationSearch"><Search :size="15"/></button><button class="icon-button" title="定位当前笔记" aria-label="定位当前笔记" :disabled="!store.active" @click="locateCurrentNote"><LocateFixed :size="15"/></button><button class="icon-button" title="展开全部目录" aria-label="展开全部目录" @click="expandDirectories"><ExpandDirectories :size="15"/></button><button class="icon-button" title="折叠全部目录" aria-label="折叠全部目录" @click="collapseDirectories"><ChevronsDownUp :size="15"/></button><button class="icon-button" title="新建笔记" @click="newNote"><Plus :size="15" /></button></div></div>

   <NavigationSearch v-if="navigationSearchOpen" ref="navigationSearch" v-model="navigationQuery" @close="closeNavigationSearch"/>
   <div class="workspace-trees" tabindex="0"><WorkspaceTree v-for="workspace in store.workspace?[store.workspace]:[]" :ref="el => setTreeRef(workspace.id,el)" :key="workspace.id" :workspace="workspace" :active="workspace.id === store.workspace?.id" :expanded="!!navigationQuery.trim() || !ui.collapsedWorkspaces.includes(workspace.id)" :query="navigationQuery" :files="navigationFiles" :selected="page === 'notes' ? selectedEntry : ''" :order="ui.entryOrder[workspace.id] || {}" @toggle="toggleWorkspace(workspace.id)" @directory="path => navigateWorkspace(workspace.id,path)" @note="(note,matchLine,newTab) => navigateWorkspaceNote(workspace.id,note,matchLine,newTab)" @reorder="(parent,paths) => orderEntries(workspace.id,parent,paths)" @move-entry="(path,isDirectory,directory) => moveDraggedEntry(workspace.id,path,isDirectory,directory)" @focus-entry="(path,isDirectory) => focusEntry(workspace.id,path,isDirectory)" @context-entry="(event,path,isDirectory) => showContext(event,workspace.id,path,isDirectory)"/></div>

    </section>

    <section v-show="sidebarTab==='trash'" id="sidebar-panel-trash" class="sidebar-trash-panel" role="tabpanel" aria-labelledby="sidebar-tab-trash">
     <div class="sidebar-section"><span>回收站</span><Trash2 :size="15"/></div>
     <TrashBrowser v-if="store.workspace" :key="store.workspace.id" :show="sidebarTab==='trash'" :workspace-id="store.workspace.id" @preview="value=>trashPreview=value"/>
     <p v-else class="muted small">请先打开一个工作区。</p>
    </section>



    <section v-show="sidebarTab==='bookmarks'" id="sidebar-panel-bookmarks" role="tabpanel" aria-labelledby="sidebar-tab-bookmarks" class="sidebar-bookmarks">

     <div class="sidebar-section"><span>书签 · {{bookmarks.length}}</span><button class="icon-button" aria-label="添加当前位置书签" :disabled="bookmarkBusy||!store.active" @mousedown.prevent @click="addBookmark"><Plus :size="15"/></button></div>

     <p class="bookmark-help">顶部书签按钮收藏整篇笔记；如需收藏具体位置，将光标放在编辑器中，点击“添加当前位置书签”或按 {{hotkey('addBookmark')}}。</p>

     <input v-model="bookmarkName" class="bookmark-input" placeholder="书签名称（可选）" maxlength="200" aria-label="新书签名称"/>

     <div class="list-search"><Search :size="15"/><input v-model="bookmarkFilter" placeholder="查找书签…"/></div>

     <p v-if="bookmarkError" class="bookmark-error" role="alert">{{bookmarkError}}</p>

     <div class="bookmark-list"><article v-for="item in visibleBookmarks" :key="item.id" class="bookmark-row"><button class="bookmark-target" @click="visitBookmark(item)"><BookmarkIcon :size="15"/><span><strong>{{item.kind==='note'?(store.data.notes.find(note=>note.id===item.noteId)?.title||item.label):item.label}}</strong><small>{{store.data.notes.find(note=>note.id===item.noteId)?.title || '笔记已删除'}} · {{item.kind==='note'?'整篇笔记':'第 '+(item.line+1)+' 行'}}</small><small v-if="item.kind!=='note'">{{item.text || '空白行'}}</small></span></button><button class="icon-button" :aria-label="'删除书签 '+item.label" @click="removeBookmark(item)"><X :size="13"/></button></article><p v-if="!visibleBookmarks.length" class="bookmark-help">{{bookmarks.length?'没有匹配的书签':'还没有书签，收藏笔记或具体位置以便快速返回。'}}</p></div>

    </section>

    <div v-show="sidebarTab!=='navigation' && sidebarTab!=='bookmarks' && sidebarTab!=='trash'" class="sidebar-tool-pages">

   <section v-show="sidebarTab==='tasks'" id="sidebar-panel-tasks" role="tabpanel" aria-labelledby="sidebar-tab-tasks" class="task-navigation"><div class="sidebar-section">待办与日程</div><button v-for="item in [{id:'day',label:'当日待办'},{id:'recent',label:'最近待办'},{id:'calendar',label:'日程概览'},{id:'recurring',label:'循环计划'},{id:'inbox',label:'待办箱'},{id:'done',label:'完成归档'},{id:'candidate',label:'候选任务'}]" :key="item.id" :class="{active:taskView===item.id && page==='tasks'}" @click="taskView=item.id;page='tasks'"><CalendarDays :size="16"/>{{item.label}}</button><p class="muted small">自动识别笔记意图，点击待办可访问原笔记。</p></section>

   <section v-show="sidebarTab === 'search'" id="sidebar-panel-search" role="tabpanel" aria-labelledby="sidebar-tab-search" class="page-content search-page"><div class="page-heading"><div><span class="eyebrow">FIND YOUR NEXT CONNECTION</span><h1>答案，或许早已记下。</h1><p>搜索本地笔记的标题、正文、标签与路径。</p></div><span class="page-icon"><Search :size="29" /></span></div><NSelect v-model:value="globalSearchKind" aria-label="全局搜索模式" :options="[{label:'语义搜索',value:'semantic'},{label:'文本查找与替换',value:'text'},{label:'正则查找与替换',value:'regex'}]"/><WorkspaceTextSearch :key="store.workspace?.id" ref="workspaceSearch" v-if="globalSearchKind!=='semantic'" v-model:query="query" :regex="globalSearchKind==='regex'" @open="(note,line)=>openNote(note,line)"/><template v-else><div class="search-box"><Search :size="21" /><NInput v-model:value="query" class="search-query" size="large" :bordered="false" placeholder="搜索一个想法、一段记录、一个关键词…" @keydown.enter="search" /><kbd>↵</kbd></div><div class="search-description"><span><ShieldCheck :size="13" />全文检索在本地运行</span><span>内置语义模型 · 关键词补充</span></div><p v-if="searchMessage" class="search-mode-message" :class="{'bookmark-error':searchMode==='keyword'}" role="status">{{searchMessage}}</p><p v-if="searching" class="muted">正在查找相关内容，首次搜索可能稍慢…</p><div v-else-if="searched" class="search-results"><div class="results-heading">找到 {{ results.length }} 篇笔记<button v-if="results.length" class="text-button" @click="prepareLLM('summary')">生成引用摘要</button></div><button v-for="result in results" :key="result.note.id" class="search-result" @click="openNote(result.note,result.line,true,$event.ctrlKey||$event.metaKey)"><div><span class="result-icon"><FileText :size="18" /></span><h3>{{ result.note.title }}</h3><ArrowUpRight :size="16" /></div><p>{{ result.snippet }}</p><footer><span v-if="result.matchType" class="search-match-kind">{{result.matchType==='keyword'?'关键词匹配':result.matchType==='both'?'语义 + 关键词':'语义相关'}}<template v-if="result.matchType!=='keyword'"> · {{(result.semanticScore || 0).toFixed(3)}}</template></span>{{ result.note.path }}<span>第 {{ result.line + 1 }} 行</span></footer></button><div v-if="!results.length" class="empty-state"><Search :size="35" /><h3>暂时没有找到相关内容</h3><p>试试更具体的描述，或检查当前工作区。</p></div></div><div v-else class="search-empty"><div class="search-decoration"><Search :size="40" :stroke-width="1" /></div><h3>在自己的知识里，发现新的线索。</h3><p>可输入关键词或一句话，按含义查找相关笔记。</p><div class="capability-notice"><span class="status-pill">已接入</span>内置模型在本机分析标题、正文、标签和路径，无需联网。</div></div></template></section>

   <section v-show="sidebarTab==='graph'" id="sidebar-panel-graph" role="tabpanel" aria-labelledby="sidebar-tab-graph" class="page-content"><h1>知识拓扑</h1><p class="bookmark-help">知识拓扑在主页面展示，关闭后回到原笔记视图。</p><button class="secondary-button" @click="openGraph">打开知识拓扑</button></section>

   <section v-show="sidebarTab === 'attachments'" id="sidebar-panel-attachments" role="tabpanel" aria-labelledby="sidebar-tab-attachments" class="page-content"><div class="page-heading"><div><span class="eyebrow">EVERY DETAIL, KEPT SAFE</span><h1>附件库</h1><p>附件集中管理，笔记之间可以共享引用。</p></div><span class="page-icon"><Paperclip :size="30" /></span></div><AttachmentLibrary :key="store.workspace?.id" :workspace-id="store.workspace?.id" :attachments="store.data.attachments" :notes="store.data.notes" :active="sidebarTab==='attachments'" :selected="attachmentMain?.attachment.id" :downloading="attachmentDownloading" :before-delete="beforeAttachmentDelete" @open="previewAttachment($event,'main')" @download="downloadAttachment" @changed="attachmentsDeleted" @message="notify" /></section>

    </div>

   </div>

   <div class="sidebar-bottom"><button @click="settingsOpen=true"><Settings2 :size="17" />设置</button><button class="icon-button" title="切换主题" @click="setTheme"><Moon v-if="!theme" :size="17" /><Sun v-else :size="17" /></button></div>

  </aside>

  <PanelDivider v-if="!collapsed" label="调整导航栏宽度" :value="layout.sidebarWidth" @change="delta=>resizePanel('sidebar',delta)" @finish="persistUI"/>

  <main class="main">

   <header class="topbar"><div class="breadcrumb"><button class="icon-button" aria-label="返回源文件" :title="'返回源文件（后退） '+hotkey('back')" :disabled="!backHistory.length || navigatingHistory || busy" @click="browseHistory('back')"><ArrowLeft :size="17"/></button><button class="icon-button" aria-label="前进" :title="'前进 '+hotkey('forward')" :disabled="!forwardHistory.length || navigatingHistory || busy" @click="browseHistory('forward')"><ArrowRight :size="17"/></button><button class="icon-button" title="收起侧栏" @click="collapsed = !collapsed"><PanelLeftClose :size="17" /></button><span>{{ store.workspace?.name || 'SmartNote' }}</span><ChevronRight :size="13" /><strong>{{ sidebarTab==='trash' ? '回收站' : ({ notes: '笔记', tasks: '待办事项', search: '搜索', graph: '知识拓扑', attachments: '附件库', settings: '设置' })[page] }}</strong></div><div class="topbar-right"><div class="layout-tools"><button class="secondary-button" aria-label="面板布局" :aria-expanded="layoutOpen" @click="layoutOpen=!layoutOpen"><Columns2 :size="15"/>面板</button><div v-if="layoutOpen" class="layout-popover" role="menu" aria-label="面板布局菜单"><span class="context-caption">显示 / 隐藏面板</span><button v-for="panel in panels" :key="panel.id" role="menuitemcheckbox" :aria-checked="panelVisible(panel.id)" @click="setPanel(panel.id,!panelVisible(panel.id))"><span>{{panel.label}}</span><Check v-if="panelVisible(panel.id)" :size="14"/></button><hr/><span class="context-caption">右侧页签</span><button role="menuitem" @click="showAllRightTabs"><span>显示全部右侧页签</span></button><button v-for="tab in orderedRightTabs" :key="tab.id" role="menuitemcheckbox" :aria-checked="rightTabs.some(item=>item.id===tab.id)" @click="rightTabs.some(item=>item.id===tab.id)?closeRightTab(tab.id):openRightTab(tab.id)"><span>{{tab.label}}</span><Check v-if="rightTabs.some(item=>item.id===tab.id)" :size="14"/></button><hr/><button role="menuitemcheckbox" :aria-checked="settingsOpen" @click="settingsOpen=!settingsOpen"><span>设置</span><Check v-if="settingsOpen" :size="14"/></button><hr/><button role="menuitem" @click="resetLayout">恢复默认布局</button><template v-if="hiddenDialogs.some(item=>item.workspaceId===store.workspace?.id) || hiddenImage || archiveMinimized"><hr/><span class="context-caption">隐藏的窗口</span><button v-for="saved in hiddenDialogs.filter(item=>item.workspaceId===store.workspace?.id)" :key="saved.id" role="menuitem" @click="restoreDialog(saved.id)">恢复{{dialogNames[saved.type]}}</button><button v-if="archiveMinimized" role="menuitem" @click="archiveOpen=true;archiveMinimized=false;layoutOpen=false">恢复智能整理</button><button v-if="hiddenImage" role="menuitem" @click="image=hiddenImage;hiddenImage='';layoutOpen=false">恢复图片查看器</button></template></div></div><button class="icon-button" title="设置" aria-label="打开设置窗口" @click="settingsOpen=true"><Settings2 :size="17"/></button><ReminderCenter :workspace-id="store.workspace?.id || ''" :revision="reminderRevision" @open="task=>{const note=store.data.notes.find(n=>n.id===task.noteId);if(note)openNote(note,task.line)}"/><SyncIndicator :workspace-id="store.workspace?.id || ''" @open="settingsTab='sync';settingsOpen=true" @changed="requestWorkspaceCheck"/><span class="topbar-divider" /><button class="icon-button" aria-label="导入笔记文件夹" :title="'导入笔记 '+hotkey('import')" :disabled="busy||store.switching" @click="importWorkspace"><Upload :size="17" /></button><button class="icon-button" aria-label="标准 Markdown 导出" title="标准 Markdown 导出" :disabled="!store.workspace || busy" @click="exportWorkspace(false)"><Download :size="17" /></button><button class="new-note-button" :disabled="busy" @click="newNote"><Plus :size="15" />新建笔记</button></div></header>


   <div v-if="!desktop" class="preview-banner">浏览器界面预览 · 文件读写请运行 SmartNote 桌面应用</div>

   <div v-if="store.error" class="error-banner"><span>{{ store.error }}</span><button class="icon-button" @click="store.error = ''"><X :size="16" /></button></div>

   <div ref="notesLayout" v-show="sidebarTab!=='trash' && sidebarTab!=='attachments'" class="notes-layout" :style="{paddingRight:rightPanelVisible?'min('+(inspectorDisplayWidth+6)+'px, 100%)':undefined}">

   <NoteTabs v-if="store.workspace" :tabs="displayedNoteTabs" :active="activeNoteTab" :disabled="noteTabBusy||store.switching||busy" @select="selectNoteTab" @close="closeNoteTab" @close-group="closeNoteTabs" @new="newNoteTab"/>
    <div class="note-workbench">
    <section v-if="sidebarTab==='navigation' && !isHidden('list')" class="note-list-panel"><div class="list-heading"><div><h1>{{ folder === '.' ? '工作区根目录' : folder || '全部笔记' }}</h1><span>{{ notes.length }} 篇笔记<span class="little-dot">·</span>让想法自然生长</span></div><div class="panel-heading-actions"><button class="icon-button" title="新建笔记" @click="newNote"><Plus :size="19" /></button><button class="icon-button" aria-label="隐藏笔记列表" title="隐藏笔记列表" @click="setPanel('list',false)"><X :size="14"/></button></div></div><div v-if="folder" class="directory-location"><button v-if="folder !== '.'" class="text-button" title="返回上级目录" @click="enterFolder(parentDirectory(currentDirectory))"><ArrowLeft :size="14" />返回上级</button><button class="text-button" @click="enterFolder('')">工作区根目录</button><span v-if="folder !== '.'">/ {{ folder }}</span></div><div class="list-search"><Search :size="15" /><input v-model="filter" placeholder="筛选笔记…" /><kbd>/</kbd></div><div class="list-sort"><span>最近编辑</span><ChevronDown :size="13" /><span class="sort-count">{{ notes.length }} 篇</span></div>

     <div class="note-list" tabindex="0"><template v-for="entry in listEntries" :key="entry.path"><button v-if="entry.directory" class="directory-card" @focus="focusEntry(store.workspace!.id,entry.directory,true)" @contextmenu.prevent="showContext($event,store.workspace!.id,entry.directory,true)" :title="'进入 ' + entry.directory" @click="enterListFolder(entry.directory)"><Folder :size="21" /><span>{{ entry.directory.slice(entry.directory.lastIndexOf('/') + 1) }}</span><ChevronRight :size="16" /></button><button v-for="note in entry.note ? [entry.note] : []" :key="note.id" class="note-card" draggable="true" @dragstart="startNoteDrag($event,store.workspace!.id,note)" @focus="focusEntry(store.workspace!.id,note.path,false)" @contextmenu.prevent="showContext($event,store.workspace!.id,note.path,false)" :class="{ selected: store.active?.id === note.id }" @click="openNote(note,undefined,true,$event.ctrlKey||$event.metaKey)"><div class="note-card-top"><span class="note-file-icon"><FileText :size="15" /></span><time>{{ new Date(note.updated).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) }}</time></div><h3>{{ note.title }}</h3><p>{{ note.content.replace(/^---[\s\S]*?---/, '').replace(/[#*`\[\]>]/g, '').trim().slice(0, 90) || '写下你的第一个想法…' }}</p><div v-if="note.tags.length" class="note-card-footer"><span v-for="tag in note.tags.slice(0, 2)" :key="tag" class="tag">{{ tag }}</span></div></button></template><div v-if="!notes.length && !childFolders.length" class="list-empty"><FileText :size="30" /><strong>{{ filter ? '没有匹配的文件或文件夹' : '当前目录为空' }}</strong><span>{{ filter ? '尝试其他关键词' : '可以在此目录创建笔记' }}</span><button v-if="!filter" class="text-button" @click="newNote">创建第一篇笔记 <ArrowUpRight :size="14" /></button></div></div>

     <div class="list-footer"><HardDrive :size="12" /> Markdown 文件 · 本地存储</div>

    </section>

    <PanelDivider v-if="sidebarTab==='navigation' && !isHidden('list')" label="调整笔记列表宽度" :value="layout.listWidth" @change="delta=>resizePanel('list',delta)" @finish="persistUI"/>

    <section v-if="store.active && activeInDirectory && (sourceVisible || previewVisible)" class="editor-panel"><header class="editor-toolbar compact-editor-toolbar" @contextmenu.prevent="showContext($event,store.workspace!.id,store.active.path,false)"><div class="note-header-meta"><strong>{{store.active.title}}</strong><time class="note-edited-at" :datetime="store.active.updated">修改于 {{noteTimestamp(store.active.updated)}}</time></div><div class="editor-tools"><button class="icon-button" :class="{'note-bookmarked':activeNoteBookmark}" :aria-label="activeNoteBookmark?'取消收藏整篇笔记':'收藏整篇笔记'" :aria-pressed="!!activeNoteBookmark" :title="activeNoteBookmark?'取消整篇笔记收藏':'收藏整篇笔记'" :disabled="bookmarkBusy" @mousedown.prevent @click="toggleNoteBookmark"><BookmarkIcon :size="16" :fill="activeNoteBookmark?'currentColor':'none'"/></button><button class="icon-button" aria-label="智能整理" title="智能整理" :disabled="busy" @click="openArchive"><Sparkles :size="16"/></button><button v-if="lastArchive && lastArchive.result.note.id===store.active.id" class="icon-button" aria-label="撤销上次整理" title="撤销上次整理（笔记修改后不可撤销）" :disabled="busy" @click="undoLastArchive"><ArrowLeft :size="16"/></button><div class="view-toggle"><button :class="{ active: mode === 'edit' }" title="编辑" @click="setMode('edit')"><Code2 :size="15" /></button><button :class="{ active: mode === 'split' }" title="分屏" @click="setMode('split')"><Columns2 :size="15" /></button><button :class="{ active: mode === 'preview' }" title="预览" @click="setMode('preview')"><Eye :size="15" /></button></div><button class="icon-button" title="插入附件" :disabled="busy" @click="fileInput?.click()"><Paperclip :size="16" /></button><button class="icon-button" title="移动或重命名笔记" @click="movePath = store.active.path; modal = 'move'"><MoreHorizontal :size="16" /></button><button class="icon-button danger-hover" title="删除笔记" @click="requestDelete(entryTarget(store.active.path,false))"><Trash2 :size="15" /></button></div></header>

     <div v-if="store.conflict" class="conflict-banner"><span>磁盘文件发生变化，自动保存已暂停。草稿已保留。</span><button class="text-button" @click="modal = 'conflict'">比较并合并 <ArrowUpRight :size="14" /></button></div>

     <div class="editor-body" :class="mode"><NoteFindBar ref="findBar" :show="findOpen" v-model:query="findQuery" :count="findCount" :index="findIndex" v-model:regex="findRegex" v-model:case-sensitive="findCase" v-model:replacement="findReplacement" v-model:replace-open="findReplaceOpen" :error="findError" @replace="replaceInNote" :target="findReplaceOpen?'Markdown 原文':activeFindTarget==='preview'?'预览':'编辑器'" @step="stepFind" @close="closeFind"/><div v-if="sourceVisible" class="source-pane" @pointerdown="findTarget='editor'" :style="{flex:previewVisible ? `${layout.splitRatio} 1 0` : '1 1 0'}"><div class="pane-label">MARKDOWN<button class="icon-button" aria-label="隐藏编辑器" title="隐藏编辑器" @click="setPanel('editor',false)"><X :size="13"/></button></div><MarkdownToolbar :shortcuts="ui.shortcuts" :disabled="busy" :metadata-visible="metadataVisible" @command="id=>editor?.command(id)" @attach="fileInput?.click()" @toggle-metadata="metadataVisible=!metadataVisible"/><MarkdownEditor :metadata-visible="metadataVisible" :note-title="store.active.title" :key="store.active.id" ref="editor" :note-path="store.active.path" :workspace-id="store.workspace!.id" @error="notify" :model-value="store.content" :line="line" @update:model-value="store.edit" @file="upload" /></div><PanelDivider v-if="sourceVisible && previewVisible" label="调整编辑器与预览比例" :value="layout.splitRatio" @change="delta=>resizePanel('split',delta)" @finish="persistUI"/><div v-if="previewVisible" class="preview-pane" @pointerdown="findTarget='preview'" :style="{flex:sourceVisible ? `${100-layout.splitRatio} 1 0` : '1 1 0'}"><div class="pane-label">预览 <Eye :size="12"/><button class="icon-button" aria-label="隐藏预览" title="隐藏预览" @click="setPanel('preview',false)"><X :size="13"/></button></div><MarkdownPreview ref="previewRef" :key="store.data.settings.activeId" :content="store.content" @link="followLink" @image="showImage" /></div></div>

     <footer class="editor-status"><span><span :class="['save-dot', { dirty: store.dirty }]" />{{ store.status }}</span><span>{{ wordCount }} 字<span class="little-dot">·</span>UTF-8<span class="little-dot">·</span>Markdown</span></footer>

    </section>

    <section v-else-if="store.active && activeInDirectory" class="panels-empty"><p>编辑器与预览已隐藏</p><button class="secondary-button" @click="setMode('split')">恢复编辑器与预览</button></section>

    <section v-else-if="store.workspace && !store.active" class="welcome-panel new-tab-page"><BookOpen :size="45" :stroke-width="1.3"/><span class="eyebrow">SMARTNOTE</span><h1>新标签页</h1><p>打开一篇笔记，继续思考。</p><div class="new-tab-actions"><button class="secondary-button" @click="newNote"><Plus :size="17"/>新建笔记<kbd>{{hotkey('newNote')}}</kbd></button><button class="secondary-button" @click="executeCommand('quickOpen')"><FileText :size="17"/>打开笔记<kbd>{{hotkey('quickOpen')}}</kbd></button><button class="secondary-button" @click="importWorkspace"><Upload :size="17"/>导入笔记</button></div></section>
    <section v-else class="welcome-panel"><div class="welcome-art"><div class="orbit orbit-one" /><div class="orbit orbit-two" /><span class="floating-note n1"><FileText :size="22" /></span><span class="floating-note n2"><ListTodo :size="20" /></span><span class="floating-note n3"><Network :size="20" /></span><div class="welcome-mark"><BookOpen :size="47" :stroke-width="1.5" /><span class="welcome-leaf"><Leaf :size="19" /></span></div></div><span class="eyebrow">A QUIET SPACE FOR YOUR MIND</span><h1>让每一个想法，都有归处。</h1><p>记录当下，连接知识。<br />在属于你的空间里，让灵感慢慢生长。</p><button class="primary-button" :disabled="busy" @click="store.workspace ? newNote() : addWorkspace()"><Plus :size="17" />{{ store.workspace ? '写下第一篇笔记' : '打开本地工作区' }}<span class="button-shortcut">Ctrl N</span></button><div class="welcome-features"><span><HardDrive :size="14" />本地优先</span><span><FileText :size="14" />开放格式</span><span><ShieldCheck :size="14" />数据自主</span></div><button class="text-button import-welcome" :disabled="busy" @click="importWorkspace"><Upload :size="14" />已有笔记？导入文件夹或 ZIP</button></section>

    </div>
    <PanelDivider v-if="rightPanelVisible" class="inspector-divider" :style="{right:'min('+inspectorDisplayWidth+'px, calc(100% - 6px))'}" label="调整右侧栏宽度" :value="inspectorDisplayWidth" @change="delta=>resizePanel('inspector',delta)" @finish="persistUI"/>
    <aside v-show="rightPanelVisible" class="inspector right-panel" :style="{width:inspectorDisplayWidth+'px'}" aria-label="笔记辅助侧栏">
     <header class="right-panel-header"><RightPanelTabs :tabs="rightTabs" :model-value="rightTab" @update:model-value="id=>rightTab=id as RightTab" @close="closeRightTab" @reorder="reorderRightTabs"/><button class="icon-button" aria-label="隐藏右侧栏" title="隐藏右侧栏，可在「面板」菜单恢复" @click="setPanel('inspector',false)"><X :size="14"/></button></header>
     <section v-show="rightTab==='outline'" id="right-content-outline" class="right-panel-content" role="tabpanel" aria-labelledby="right-tab-outline" tabindex="0"><div class="inspector-heading">笔记大纲<span class="right-panel-count">{{outline.length}}</span></div><button v-for="heading in outline" :key="heading.line" class="outline-item" :style="{ paddingLeft: (heading.level - 1) * 10 + 4 + 'px' }" @click="jumpOutline(heading.line)">{{ heading.text }}</button><p v-if="!outline.length" class="muted small">使用 Markdown 标题组织内容</p></section>
     <section v-show="rightTab==='ai'" id="right-content-ai" class="right-panel-chat" role="tabpanel" aria-labelledby="right-tab-ai"><AIAssistant :visible="aiOpen&&rightPanelVisible" :panel-hidden="!rightPanelVisible" :workspace-id="store.workspace?.id||''" :note="store.active" :revision="llmSettingsRevision" :selection="aiSelection" :prepare-context="prepareAIContext" :apply-edit="applyAIEdit" @close="aiOpen=false" @settings="settingsTab='llm';settingsOpen=true" @source="openAISource" @clear-selection="aiSelection=''"/></section>
     <section v-show="rightTab==='related'" id="right-content-related" class="right-panel-content" role="tabpanel" aria-labelledby="right-tab-related" tabindex="0"><div class="inspector-heading">关联笔记<span class="right-panel-count">{{related.length}}</span></div><button v-for="note in related" :key="note.id" class="related-note" @click="openNote(note,undefined,true,$event.ctrlKey||$event.metaKey)"><FileText :size="14" /><span>{{ note.title }}</span><ArrowUpRight :size="12" /></button><p v-if="!related.length" class="muted small">添加相同标签、项目或双链，建立知识联系。</p><button class="graph-entry" @click="openGraph"><Network :size="15" />查看知识拓扑<ArrowUpRight :size="12" /></button><div class="inspector-tip"><Sparkles :size="16" /><p>知识的价值，<br />藏在彼此的连接里。</p></div></section>
    </aside>

   </div>

   <section v-if="sidebarTab==='attachments' && !graphOpen" class="attachment-main-view" role="region" aria-label="主区域附件预览"><AttachmentContent v-if="attachmentMain" :attachment="attachmentMain.attachment" :preview="attachmentMain.preview" :loading="attachmentMain.loading" :error="attachmentMain.error" :downloading="attachmentDownloading" @download="downloadAttachment(attachmentMain!.attachment,attachmentMain!.workspaceId)"/><div v-else class="panels-empty"><Paperclip :size="38"/><h2>附件库</h2><p>点击左侧附件，在这里查看内容。</p></div></section>

   <section v-if="sidebarTab==='trash' && !graphOpen" class="trash-main-view" role="region" aria-label="回收笔记预览">
    <template v-if="trashPreview"><header class="trash-main-header"><div><h1>{{trashPreview.entry.name}}</h1><p>{{trashPreview.path}} <span class="status-pill">回收站 · 只读</span></p></div><button class="icon-button" title="关闭回收笔记预览" aria-label="关闭回收笔记预览" @click="trashPreview=undefined"><X :size="17"/></button></header><p v-if="trashPreview.loading" class="trash-main-status">正在读取笔记…</p><p v-else-if="trashPreview.error" class="trash-main-status" role="alert">{{trashPreview.error}}</p><div v-else class="preview-pane trash-main-content"><MarkdownPreview :content="trashPreview.content" @image="src=>image=src" @link="notify('回收笔记为只读预览')"/></div></template>
    <div v-else class="panels-empty"><Trash2 :size="38"/><h2>回收站</h2><p>点击左侧回收文件，在这里查看内容。</p></div>
   </section>

   <TodoView :key="store.workspace?.id" ref="todo" v-show="page==='tasks' && !graphOpen" class="task-main-overlay" :view="taskView" @minimize-task="minimizeDialog" @open="openNote" />

   <section v-if="graphInitialized" v-show="graphOpen" role="region" aria-label="主页面知识拓扑" class="graph-page graph-main-overlay"><div class="graph-heading"><div><span class="eyebrow">IDEAS ARE BETTER CONNECTED</span><h1>知识拓扑</h1><p>让分散的记录，连成属于你的知识网络。</p></div><div class="graph-controls"><button class="secondary-button" :class="{active:graphLinking}" :aria-pressed="graphLinking" @click="graphLinking=!graphLinking">{{graphLinking?'取消连线':'手动连线'}}</button><button class="secondary-button" :aria-expanded="graphRelationsVisible" @click="manualGraph">{{graphRelationsVisible?'隐藏关联面板':'手动关联管理'}}</button><button class="secondary-button" aria-label="关闭知识拓扑" @click="closeGraph"><X :size="16"/>关闭</button><div class="segmented"><button :class="{active: !globalGraph}" @click="globalGraph = false">局部</button><button :class="{active: globalGraph}" @click="globalGraph = true">全局</button></div><NSelect v-model:value="relationFilter" :options="[{label:'全部关系',value:''},{label:'笔记链接',value:'MARKDOWN_LINK'},{label:'手动关联',value:'USER_CONFIRMED'}]" style="width:140px" /></div></div><div class="graph-workbench"><KnowledgeGraph :workspace-id="store.workspace?.id" :key="store.workspace?.id" ref="graphView" :notes="store.data.notes" :edges="graphEdges" :annotations="graphAnnotations.nodes" :revision="graphAnnotations.revision" :active="graphOpen" @changed="metadata=>{graphAnnotations=metadata;refreshGraph()}" :linking="graphLinking" :selected="store.active?.id" :global="globalGraph" :filter="relationFilter" @open="openGraphNote" @inspect="inspectGraphEdge" @inspect-node="inspectGraphNode" @connect="connectGraph" /><GraphRelations v-if="graphRelationsVisible" :workspace-id="store.workspace?.id" :key="store.workspace?.id" :notes="store.data.notes" :edges="graphEdges" :annotations="graphAnnotations" :selected="store.active?.id" :inspected="inspectedGraphEdge" :node="inspectedGraphNode" :pair="graphPair" @changed="refreshGraph" @open="openGraphNote" @close="graphRelationsVisible=false" /></div><div v-if="!store.data.notes.length" class="graph-empty"><Network :size="40" /><h3>从第一篇笔记，开始连接</h3><p>笔记链接与手动关联会在这里展示。</p></div><div class="graph-legend"><span><i />笔记</span><span>{{ graphEdges.length }} 条真实关联</span><span class="confirmed-legend">绿色：手动关联</span><span>滚轮缩放 · 拖动节点 · 选中后空格编辑备注 · 双击节点或标题打开 · Enter 打开</span></div></section>

  </main>

 </div>

 <Teleport to="body"><div v-if="store.switching" class="workspace-switch-shield" role="status" aria-live="polite"><span>{{folderImportOpen?'正在加载笔记和附件，请稍候…':'正在切换工作区…'}}</span></div></Teleport>
 <FolderImportDialog v-model:show="folderImportOpen" @zip="importZipWorkspace"/>
 <LLMReview :show="llmOpen" :input="llmInput" @close="llmOpen=false" @apply="applyLLM" @source="openLLMSource" @settings="llmOpen=false;settingsTab='llm';settingsOpen=true"/>
 <FloatingWindow :show="settingsOpen" title="设置" @close="settingsOpen=false" @minimize="settingsOpen=false">   <section class="settings-page"><div class="settings-nav"><h1>设置</h1><button v-for="tab in [{id:'general',name:'通用设置'},{id:'shortcuts',name:'快捷键'},{id:'workspace',name:'工作区与备份'},{id:'archive',name:'智能归档'},{id:'picgo',name:'附件与 PicGo'},{id:'model',name:'搜索与模型'},{id:'reminders',name:'任务提醒'},{id:'sync',name:'WebDAV 同步'},{id:'llm',name:'大模型配置'},{id:'about',name:'关于 SmartNote'}]" :key="tab.id" :class="{active: settingsTab === tab.id}" @click="settingsTab = tab.id">{{ tab.name }}</button></div><div class="settings-detail">

    <template v-if="settingsTab === 'general'"><span class="eyebrow">MAKE YOURSELF AT HOME</span><h1>通用设置</h1><p>一个适合安静思考的空间。</p><div class="setting-row"><div><strong>外观主题</strong><p>切换浅色与深色界面</p></div><button class="secondary-button" @click="setTheme"><Moon v-if="!theme" :size="15" /><Sun v-else :size="15" />{{ theme ? '深色模式' : '浅色模式' }}</button></div><div class="setting-row"><div><strong>自动保存</strong><p>停止输入 800ms 后保存，修改前校验磁盘内容</p></div><span class="status-pill">已开启</span></div><div class="setting-row"><div><strong>本地草稿恢复</strong><p>未保存内容保留在当前设备的应用存储中</p></div><span class="status-pill">已开启</span></div><div class="setting-row"><div><strong>界面字体</strong><p>调整导航、列表和设置文字；字体需已安装</p></div><select aria-label="界面字体" class="font-family-select" :value="ui.fontFamily || 'system'" @change="setFontFamily(($event.target as HTMLSelectElement).value)"><option v-for="option in fontOptions" :key="option.value" :value="option.value" :style="{fontFamily:fontStack(option.value)}">{{option.label}}</option></select></div><div class="setting-row"><div><strong>界面字体大小</strong><p>调整导航、列表和设置文字，自动保存</p></div><div class="font-size-control"><input aria-label="界面字体大小" type="range" min="12" max="22" :value="ui.fontSize" @input="setFontSize(Number(($event.target as HTMLInputElement).value))"/><output>{{ui.fontSize}} px</output><button class="text-button" @click="setFontSize(14)">重置</button></div></div><div class="setting-row"><div><strong>编辑器字体</strong><p>用于笔记编辑和正文预览，独立于界面字体；字体需已安装</p></div><select aria-label="编辑器字体" class="font-family-select" :value="ui.editorFontFamily" @change="setEditorFontFamily(($event.target as HTMLSelectElement).value)"><option v-for="option in fontOptions" :key="option.value" :value="option.value">{{option.label}}</option></select></div><div class="setting-row"><div><strong>编辑器字体大小</strong><p>编辑和预览使用相同的正文字号</p></div><div class="font-size-control"><input aria-label="编辑器字体大小" type="range" min="10" max="48" :value="ui.editorFontSize" @input="setEditorFontSize(Number(($event.target as HTMLInputElement).value))"/><output>{{ui.editorFontSize}} px</output><button class="text-button" @click="setEditorFontSize(14)">重置</button></div></div></template>

    <template v-else-if="settingsTab==='shortcuts'"><h1>快捷键</h1><p class="shortcut-help">点击按键后直接录入新组合；Backspace 清空，Esc 取消。删除、重命名仅在文件导航获得焦点时生效；正文编辑保留常用复制粘贴。</p><p v-if="shortcutError" role="alert" class="shortcut-error">{{shortcutError}}</p><div v-for="command in commands" :key="command.id" class="shortcut-row"><span>{{command.label}}<small>{{command.scope==='explorer'?'文件导航':command.scope==='editor'?'编辑器':'全局'}}</small></span><div class="shortcut-controls"><button class="shortcut-key" :class="{recording:shortcutCapture===command.id}" :aria-label="'修改快捷键 '+command.label" @click="startCapture(command.id)">{{shortcutCapture===command.id?'请按快捷键…':hotkey(command.id) || '未绑定'}}</button><button class="text-button" :aria-label="'重置快捷键 '+command.label" @click="resetShortcut(command.id)">重置</button></div></div><button class="secondary-button" @click="resetShortcut()">恢复全部默认快捷键</button></template>

    <template v-else-if="settingsTab === 'workspace'"><span class="eyebrow">YOUR DATA, ALWAYS YOURS</span><h1>工作区与备份</h1><p>开放格式迁移，原生备份恢复。</p><div class="setting-row"><div><strong>标准 Markdown 导出</strong><p>保留目录结构，附件转换为相对路径；远程图片保留 URL</p></div><NButton :disabled="!store.workspace || busy" @click="exportWorkspace(false)">导出 ZIP</NButton></div><div class="setting-row"><div><strong>SmartNote 原生备份</strong><p>笔记、附件、任务元数据和哈希校验清单</p></div><NButton :disabled="!store.workspace || busy" @click="exportWorkspace(true)">创建备份</NButton></div><div class="setting-row"><div><strong>ZIP 导入与恢复</strong><p>恢复到新目录，校验通过后注册独立工作区</p></div><NButton :disabled="busy" @click="importZipWorkspace">选择 ZIP</NButton></div><div class="info-box"><ShieldCheck :size="20" /><p>原生恢复保留笔记和附件 ID，为新工作区分配独立 ID。当前版本不下载远程图床图片，导出不等同于远程图片离线归档。</p></div></template>

    <template v-else-if="settingsTab === 'archive'"><h1>智能归档</h1></template>

    <template v-else-if="settingsTab === 'picgo'"><span class="eyebrow">SMALL DETAILS, SAFELY STORED</span><h1>附件与 PicGo</h1><p>截图上传与本地保存，按你的习惯选择。</p>
     <div class="setting-row"><div><strong>PicGo 上传</strong><p>仅粘贴剪贴板截图时上传到本机 PicGo；普通图片和文件始终保存本地。</p></div><NSwitch v-model:value="attachmentOptions.picGoEnabled" aria-label="PicGo 上传"/></div>
     <label class="field-label">PicGo HTTP Server 地址</label><NInput v-model:value="picgo" :disabled="!attachmentOptions.picGoEnabled" placeholder="http://127.0.0.1:36677" :input-props="{'aria-label':'PicGo HTTP Server 地址'}"/><p class="field-help">留空使用本机默认地址 http://127.0.0.1:36677。请先启动 PicGo 的 HTTP Server 并配置图床；上传成功插入图床链接，失败自动回退本地。复制、拖入或选择的文件和普通图片保存在本地。</p>
     <div class="setting-row"><div><strong>本地原图留存</strong><p>上传成功后仍保留本地原图。关闭后，新粘贴截图上传成功时仅保存图床链接；上传失败仍保存在本地。已有原图不删除。</p></div><NSwitch v-model:value="attachmentOptions.keepOriginal" aria-label="本地原图留存"/></div>
     <div class="setting-row"><div><strong>附件去重</strong><p>同一工作区内容相同的附件复用已有记录。关闭后每次导入都创建独立附件。</p></div><NSwitch v-model:value="attachmentOptions.deduplicate" aria-label="附件去重"/></div>
     <p class="field-help">设置保存后对之后导入的附件生效；仅保留图床链接的图片需要联网查看。</p><NButton type="primary" :loading="busy" @click="saveAttachmentOptions">保存配置</NButton>
    </template>

    <template v-else-if="settingsTab === 'model'"><span class="eyebrow">LOCAL INTELLIGENCE</span><h1>搜索与模型</h1><p>记录、检索和语义归档均可离线使用。</p><div class="setting-row"><div><strong>SQLite FTS5</strong><p>关键词检索，短中文查询使用包含匹配补充</p></div><span class="status-pill">已接入</span></div><div class="setting-row"><div><strong>内置中文语义模型</strong><p>BGE-small-zh-v1.5 · 512 维 · CPU 离线推理</p></div><span class="status-pill">随程序内置</span></div><div class="info-box"><Sparkles :size="19"/><p>模型和运行库随程序提供，首次使用时自动准备，无需下载或配置。语义分类在“智能归档”中管理；搜索使用内置模型查找相关笔记，并保留关键词匹配。检索门槛与归档设置独立。</p></div><NButton :loading="busy" @click="run(async()=>{const result=await backend().CheckEmbedding();notify(result.message)})">检查内置模型</NButton><IndexSettings :workspace-id="store.workspace?.id || ''"/></template>

    <template v-else-if="settingsTab === 'reminders'"><h1>任务提醒</h1><ReminderSettings :workspace-id="store.workspace?.id || ''" @saved="reminderRevision++"/></template>

    <template v-else-if="settingsTab === 'sync'"><h1>WebDAV 多设备同步</h1><SyncSettings :key="store.workspace?.id" :workspace-id="store.workspace?.id || ''" @changed="requestWorkspaceCheck"/></template>
    <template v-else-if="settingsTab === 'llm'"><h1>大模型配置</h1><LLMSettings @saved="llmSettingsRevision++"/></template>

    <template v-else><div class="about-brand"><span class="brand-mark"><BookOpen :size="28" /></span><h1>SmartNote<span class="brand-dot">.</span></h1></div><p>让每一个想法，都有归处。</p><span class="status-pill">v{{ appVersion }} · 本地智能知识库</span><div class="setting-row"><div><strong>技术栈</strong><p>Wails · Go · Vue 3 · TypeScript · SQLite</p></div></div><div class="setting-row"><div><strong>数据位置</strong><p>笔记保存在工作区；设置、附件和索引位于：{{ actualDataDirectory || '桌面启动后显示实际数据目录' }}</p></div></div><div class="info-box"><Leaf :size="20" /><p>记录方便、任务可追溯、数据可迁移。后续阶段的进度详见项目 README 和开发状态文档。</p></div></template>

   <KeepAlive><SemanticSettings @saved="semanticConfigRevision++" @batch-applied="archiveBatchApplied" @batch-undone="archiveBatchUndone" v-if="settingsTab==='archive'" :workspace-id="store.workspace?.id || ''" :directories="folders"/></KeepAlive><KeepAlive><ArchiveRules v-if="settingsTab==='archive'" :workspace-id="store.workspace?.id || ''" :directories="folders"/></KeepAlive></div></section></FloatingWindow>

 <ArchivePanel :config-revision="semanticConfigRevision" v-model:show="archiveOpen" :workspace-id="store.workspace?.id || ''" :note="store.active" :directories="folders" @minimize="archiveOpen=false;archiveMinimized=true" @applied="archiveApplied"/>

 <div v-if="contextMenu" class="entry-context-menu" role="menu" aria-label="文件操作" :style="{left:contextMenu.x+'px',top:contextMenu.y+'px',visibility:contextMenu.ready?'visible':'hidden'}" @contextmenu.prevent @keydown="menuKeys">

  <template v-if="contextMenu.target.isDirectory">
   <button role="menuitem" @click="newNoteAt(contextMenu.target)"><span>新建笔记</span><kbd>{{hotkey('newNote')}}</kbd></button>
   <button role="menuitem" @click="requestFolder(contextMenu.target)"><span>新建文件夹</span><kbd>{{hotkey('newFolder')}}</kbd></button>
  </template>
  <template v-else>
   <button role="menuitem" @click="openContextNote(contextMenu.target)"><span>打开</span></button>
   <button role="menuitem" @click="openContextNote(contextMenu.target,true)"><span>在新标签页中打开</span></button>
  </template>

  <hr/>

  <button role="menuitem" :disabled="!contextMenu.target.path" @click="requestRename(contextMenu.target)"><span>重命名</span><kbd>{{hotkey('rename')}}</kbd></button>

  <button role="menuitem" :disabled="!contextMenu.target.path||busy" @click="copyEntry(contextMenu.target)"><span>复制</span><kbd>{{hotkey('copyEntry')}}</kbd></button>
  <button role="menuitem" :disabled="!copiedEntry||copiedEntry.workspaceId!==contextMenu.target.workspaceId||busy" @click="pasteEntry(contextMenu.target)"><span>粘贴</span><kbd>{{hotkey('pasteEntry')}}</kbd></button>

  <hr/>

  <button role="menuitem" @click="copyEntryPath(false,contextMenu.target)"><span>复制相对路径</span><kbd>{{hotkey('copyPath')}}</kbd></button>

  <button role="menuitem" @click="copyEntryPath(true,contextMenu.target)"><span>复制完整路径</span></button>

  <button role="menuitem" @click="revealEntry(contextMenu.target)"><span>在资源管理器中显示</span><kbd>{{hotkey('reveal')}}</kbd></button>


  <hr/>

  <button v-if="!contextMenu.target.isDirectory && store.data.notes.some(n=>n.path===contextMenu!.target.path&&n.automationIgnored)" role="menuitem" @click="restoreNoteAutomation(contextMenu.target)"><span>恢复待办识别与自动分类</span></button>
  <button role="menuitem" class="danger-hover" :disabled="!contextMenu.target.path" @click="requestDelete(contextMenu.target)"><span>{{contextMenu.target.isDirectory?'删除文件夹':'删除笔记'}}</span><kbd>{{hotkey('delete')}}</kbd></button>

 </div>

 <ResizableModal @minimize="minimizeDialog()" :show="modal==='entryRename'" preset="card" title="重命名" style="width:460px" @update:show="modal=null"><label class="field-label">{{operationTarget?.isDirectory?'文件夹名称':'文件名称'}}</label><NInput v-model:value="entryName" autofocus :input-props="{'aria-label':'新名称'}" @keydown.enter="renameEntry"/><p class="field-help">保留原目录；文件可省略 .md。重命名会更新相关笔记链接。</p><p v-if="store.error" role="alert" class="shortcut-error">{{store.error}}</p><template #footer><div class="modal-actions"><NButton @click="modal=null">取消</NButton><NButton type="primary" :disabled="busy" @click="renameEntry">确认重命名</NButton></div></template></ResizableModal>

 <ResizableModal @minimize="minimizeDialog()" :show="modal==='folder'" preset="card" title="新建文件夹" style="width:460px" @update:show="modal=null"><label class="field-label">文件夹名称</label><NInput v-model:value="entryName" autofocus placeholder="例如：项目/设计" :input-props="{'aria-label':'文件夹名称'}" @keydown.enter="createFolder"/><p class="field-help">创建于 {{operationTarget?.path || '工作区根目录'}}，可用 / 创建多层目录。</p><p v-if="store.error" role="alert" class="shortcut-error">{{store.error}}</p><template #footer><div class="modal-actions"><NButton @click="modal=null">取消</NButton><NButton type="primary" :disabled="busy" @click="createFolder">创建文件夹</NButton></div></template></ResizableModal>

 <ResizableModal @minimize="minimizeDialog()" :show="modal==='entryDelete'" preset="card" title="移入回收目录？" style="width:480px" @update:show="modal=null"><p class="delete-target">{{operationTarget?.path}}</p><p class="field-help">{{operationTarget?.isDirectory?'文件夹及其全部内容会移入工作区的 .smartnote-trash 隐藏目录，可从资源管理器恢复。':'笔记会移入应用数据目录的 trash，共享附件保留。'}}指向这些文件的链接会保留。</p><p class="field-help" style="overflow-wrap:anywhere">回收位置：{{operationTarget?.isDirectory ? (store.workspace?.path || '工作区')+'/.smartnote-trash/' : (actualDataDirectory || '应用数据目录')+'/workspaces/'+(store.workspace?.id || '工作区ID')+'/trash/'}}（按删除批次保留原路径）</p><p v-if="store.error" role="alert" class="shortcut-error">{{store.error}}</p><template #footer><div class="modal-actions"><NButton @click="modal=null">取消</NButton><NButton type="error" :disabled="busy" @click="deleteEntry">移入回收目录</NButton></div></template></ResizableModal>

 <ResizableModal @minimize="minimizeDialog()" :show="modal==='palette' || modal==='quickOpen'" preset="card" :title="modal==='palette'?'命令面板':'快速打开笔记'" style="width:600px" @update:show="modal=null"><NInput v-model:value="commandQuery" autofocus :input-props="{'aria-label':modal==='palette'?'搜索命令':'快速查找笔记'}" :placeholder="modal==='palette'?'输入命令名称…':'输入标题或路径…'" @keydown="paletteKeys"/><div class="command-results"><template v-if="modal==='palette'"><button v-for="(command,index) in paletteCommands" :key="command.id" :class="{chosen:index===commandIndex}" @focus="commandIndex=index" @click="runPalette(command.id)"><span>{{command.label}}</span><kbd>{{hotkey(command.id)}}</kbd></button><p v-if="!paletteCommands.length">没有匹配命令</p></template><template v-else><button v-for="(note,index) in quickNotes" :key="note.id" :class="{chosen:index===commandIndex}" @focus="commandIndex=index" @click="openTreeNote(note);modal=null"><span>{{note.title}}<small>{{note.path}}</small></span></button><p v-if="!quickNotes.length">没有匹配笔记</p></template></div></ResizableModal>

 <input ref="fileInput" hidden type="file" multiple @change="fileChanged" />

 <div v-if="toast" class="toast"><Check :size="16" />{{ toast }}</div><div v-if="busy" class="busy-badge"><span class="spinner" />正在处理…</div>

 <ResizableModal @minimize="minimizeDialog()" :show="modal === 'note'" preset="card" title="给新想法一个名字" style="width:460px" @update:show="modal = null"><label class="field-label">笔记标题</label><NInput v-model:value="noteTitle" autofocus placeholder="例如：一个值得记录的想法" @keydown.enter="createNote" /><label class="field-label">保存目录（工作区内）</label><DirectoryPicker v-model="noteDir" :directories="folders" /><p class="field-help">选择工作区内已有文件夹，或用 / 分隔输入子目录，不存在时自动创建。输入 . 保存到工作区根目录。</p><template #footer><div class="modal-actions"><NButton @click="modal = null">取消</NButton><NButton type="primary" :loading="busy" :disabled="busy" @click="createNote">创建笔记</NButton></div></template></ResizableModal>

 <ResizableModal @minimize="minimizeDialog()" :show="modal === 'move'" preset="card" title="移动或重命名笔记" style="width:520px" @update:show="modal = null"><label class="field-label">工作区内的目标路径</label><NInput v-model:value="movePath" placeholder="技术知识/新的笔记名称.md" /><p class="field-help">会更新当前笔记的相对链接，以及其他笔记指向它的 Markdown、引用式和双链链接。目标不能已存在。操作前保留原文恢复副本；没有 ID 的笔记会添加稳定 ID。</p><template #footer><div class="modal-actions"><NButton @click="modal=null">取消</NButton><NButton type="primary" :loading="busy" @click="moveNote">移动并更新链接</NButton></div></template></ResizableModal>

 <ResizableModal @minimize="minimizeDialog()" :show="modal === 'remove'" preset="dialog" title="移除当前工作区注册？" positive-text="移除注册" negative-text="取消" @positive-click="run(async () => { await store.save(); if (store.dirty) return; await backend().RemoveWorkspace(store.workspace!.id); store.active = null; modal = null; await store.refresh() })" @negative-click="modal = null" @close="modal = null">仅移除应用中的注册记录。原始笔记、附件和元数据均保留。</ResizableModal>

 <ResizableModal @minimize="minimizeDialog()" :show="modal === 'rename'" preset="card" title="重命名工作区" style="width:430px" @update:show="modal = null"><NInput v-model:value="rename" /><template #footer><NButton type="primary" @click="run(async () => { await backend().RenameWorkspace(store.workspace!.id, rename); modal = null; await store.refresh() })">保存名称</NButton></template></ResizableModal>

 <ResizableModal @minimize="minimizeDialog()" :show="modal === 'conflict'" preset="card" title="外部修改冲突" style="width:850px" @update:show="modal = null"><p>当前草稿保留在下方。可以复制草稿、载入磁盘版本，或手动编辑下方合并稿后保存。</p><div class="conflict-comparison"><div><h4>磁盘版本</h4><pre>{{ store.data.notes.find(n => n.id === store.active?.id)?.content || '文件已被移除' }}</pre></div><div><h4>当前草稿 / 合并稿</h4><NInput :value="store.content" type="textarea" :rows="15" @update:value="store.edit" /></div></div><template #footer><div class="modal-actions"><NButton @click="run(async () => { await copyDraft(); notify('草稿已复制') })">复制草稿</NButton><NButton @click="run(async () => { await store.reload(); modal = null })">载入磁盘版本</NButton><NButton type="primary" @click="run(async () => { await store.merge(store.content); if (!store.conflict && !store.dirty) modal = null })">保存合并稿</NButton></div></template></ResizableModal>

 <ImageViewer v-if="image" :src="image" :name="imageAttachment?.name" :downloadable="!!imageAttachment" :downloading="attachmentDownloading" @close="image=''" @minimize="hiddenImage=image;image=''" @download="imageAttachment && downloadAttachment(imageAttachment)" />

 <ResizableModal :show="!!attachmentModal" title="附件预览" preset="card" style="width:860px;max-width:calc(100vw - 40px)" @update:show="value=>{if(!value)closeAttachmentModal()}" @minimize="closeAttachmentModal"><div style="height:60vh;min-height:220px"><AttachmentContent v-if="attachmentModal" :attachment="attachmentModal.attachment" :preview="attachmentModal.preview" :loading="attachmentModal.loading" :error="attachmentModal.error" :downloading="attachmentDownloading" @download="downloadAttachment(attachmentModal!.attachment,attachmentModal!.workspaceId)"/></div></ResizableModal>
 <TaskIntentConfirmation v-if="!folderImportOpen" :key="store.workspace?.id" :workspace-id="store.workspace?.id || ''" @open="(note,line)=>openNote(note,line)"/>
 </NConfigProvider>

</template>

