<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { startNoteDrag } from '../services/note-drag'
import { ChevronDown, ChevronRight, Folder, FileText, ArrowUp, ArrowDown, GripVertical } from 'lucide-vue-next'
import { api, type Workspace, type WorkspaceFiles, type Note } from '../services/api'
import {navigationMatch,navigationHighlight} from '../services/navigation-match'

const props = defineProps<{ query?:string;workspace: Workspace; active: boolean; expanded: boolean; files?: WorkspaceFiles; selected: string; order: Record<string, string[]> }>()
const emit = defineEmits<{ toggle: []; directory: [path: string]; note: [note: Note,line?:number,newTab?:boolean]; reorder: [parent: string, paths: string[]]; moveEntry: [path: string, isDirectory: boolean, directory: string]; focusEntry: [path:string, isDirectory:boolean]; contextEntry: [event:MouseEvent,path:string,isDirectory:boolean] }>()
const cached = ref<WorkspaceFiles>({ notes: [], directories: [] }), loading = ref(false), error = ref('')
const closed = ref(new Set<string>()), dragging = ref(''), allCollapsed=ref(true)
const dropTarget = ref<string | null>(null)
const parentOf = (path: string) => path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : ''
type Row = { path: string; name: string; depth: number; note?: Note }
const files = computed(() => props.files || cached.value)
const siblings = computed(() => {
 const dirs = new Set(files.value.directories || [])
 for (const note of files.value.notes) { const parts = note.path.split('/'); parts.pop(); while(parts.length) { dirs.add(parts.join('/')); parts.pop() } }
 const groups = new Map<string, Row[]>()
 const add = (path: string, note?: Note) => { const parent = parentOf(path), rows = groups.get(parent) || [], filename=path.split('/').pop()!; rows.push({path, name:note?filename.replace(/\.md$/i,''):filename, depth:path.split('/').length-1, note}); groups.set(parent,rows) }
 dirs.forEach(path => add(path)); files.value.notes.forEach(note => add(note.path,note))
 for(const [parent, rows] of groups) {
  const order = Array.isArray(props.order[parent]) ? props.order[parent]! : []
  rows.sort((a,b) => { const ai=order.indexOf(a.path), bi=order.indexOf(b.path); if(ai>=0 || bi>=0) return (ai<0?Infinity:ai)-(bi<0?Infinity:bi); return Number(!!a.note)-Number(!!b.note) || a.name.localeCompare(b.name,'zh-CN') })
 }
 return groups
})
const searchTerm=computed(()=>(props.query||'').trim().toLocaleLowerCase())
const matches=computed(()=>new Map(files.value.notes.map(note=>[note.id,navigationMatch(note,props.query||'')])))
const highlights=(text:string)=>navigationHighlight(text,props.query||'')
function openRow(event:MouseEvent|KeyboardEvent,note:Note){emit('note',note,searchTerm.value?matches.value.get(note.id)?.line:undefined,event.ctrlKey||event.metaKey)}
const matchingPaths=computed(()=>{
 const paths=new Set<string>(),term=searchTerm.value
 if(!term)return paths
 for(const note of files.value.notes){
  if(!matches.value.get(note.id))continue
  let path=note.path;paths.add(path)
  while(path.includes('/')){path=parentOf(path);paths.add(path)}
 }
 return paths
})
const folderClosed=(path:string)=>!searchTerm.value&&closed.value.has(path)
const rows = computed(() => { const result: Row[]=[]; const visit=(parent:string) => { for(const row of siblings.value.get(parent) || []) { if(searchTerm.value&&!matchingPaths.value.has(row.path))continue;result.push(row); if(!row.note && !folderClosed(row.path)) visit(row.path) } }; visit(''); return result })

let pending: Promise<void>|null = null, loadedOnce = false
function load(): Promise<void> {
 if(!props.expanded || props.files) return Promise.resolve()
 if(pending) return pending
 pending=(async () => { loading.value=true; error.value=''; try { cached.value=await api.ListWorkspaceFiles(props.workspace.id); loadedOnce=true } catch(e) { error.value=String(e) } finally { loading.value=false; pending=null } })()
 return pending
}
// Same as load(), but reuses already fetched data instead of refreshing on every call (used by reveal).
function ensureLoaded(): Promise<void> { return props.files || (loadedOnce && !pending) ? Promise.resolve() : load() }
watch(() => [props.expanded, props.active], () => void load(), {immediate:true})
function toggle(path:string) { if(searchTerm.value)return;allCollapsed.value=false; const next=new Set(closed.value); if(next.has(path)) next.delete(path); else next.add(path); closed.value=next }
function move(path:string, step:number) { const parent=parentOf(path), paths=(siblings.value.get(parent)||[]).map(row=>row.path), index=paths.indexOf(path), target=index+step; if(index<0 || target<0 || target>=paths.length) return; paths.splice(index,1); paths.splice(target,0,path); emit('reorder',parent,paths) }
function drop(target:string) { const source=dragging.value; dragging.value=''; if(!source || source===target || parentOf(source)!==parentOf(target)) return; const parent=parentOf(target), paths=(siblings.value.get(parent)||[]).map(row=>row.path).filter(path=>path!==source); paths.splice(paths.indexOf(target),0,source); emit('reorder',parent,paths) }
function dropRow(row:Row) { if(!row.note && canMoveInto(row.path)) dropIntoFolder(row.path); else drop(row.path) }
function startDrag(event:DragEvent,row:Row) {dragging.value=row.path;if(row.note) startNoteDrag(event,props.workspace.id,row.note);else event.dataTransfer?.setData('text/plain',row.path)}
function endDrag() { dragging.value=''; dropTarget.value=null }
function canMoveInto(directory:string) {
 const source=dragging.value
 if(!source || parentOf(source)===directory || source===directory) return false
 const isDirectory=!files.value.notes.some(note=>note.path===source)
 return !isDirectory || !directory.startsWith(source+'/')
}
function overFolder(event:DragEvent,directory:string) { if(canMoveInto(directory)) { event.preventDefault(); dropTarget.value=directory; if(event.dataTransfer) event.dataTransfer.dropEffect='move' } }
function dropIntoFolder(directory:string) {
 if(!canMoveInto(directory)) { endDrag(); return }
 const source=dragging.value,isDirectory=!files.value.notes.some(note=>note.path===source)
 endDrag(); const next=new Set(closed.value); next.delete(directory); closed.value=next
 emit('moveEntry',source,isDirectory,directory)
}
function treeKeys(event:KeyboardEvent,row:Row) {
 if(!['ArrowUp','ArrowDown','ArrowLeft','ArrowRight'].includes(event.key)) return
 event.preventDefault()
 const index=rows.value.findIndex(item=>item.path===row.path)
 let target:Row|undefined
 if(event.key==='ArrowUp') target=rows.value[index-1]
 if(event.key==='ArrowDown') target=rows.value[index+1]
 if(event.key==='ArrowLeft') { if(!row.note && !folderClosed(row.path)) { toggle(row.path); return }; target=rows.value.find(item=>item.path===parentOf(row.path)) }
 if(event.key==='ArrowRight' && !row.note) { if(folderClosed(row.path)) { toggle(row.path); return }; target=rows.value[index+1] }
 if(target) (Array.from((event.currentTarget as HTMLElement).parentElement!.children).find(item=>(item as HTMLElement).dataset.entryPath===target!.path) as HTMLElement|undefined)?.focus()
}
function scrollDuringDrag(event:DragEvent) { if(!dragging.value) return; const container=(event.currentTarget as HTMLElement).closest('.workspace-trees'); if(!container) return; const bounds=container.getBoundingClientRect(); if(event.clientY<bounds.top+32) container.scrollTop-=14; else if(event.clientY>bounds.bottom-32) container.scrollTop+=14 }
function atEdge(row:Row, step:number) { const group=siblings.value.get(parentOf(row.path))||[]; const index=group.findIndex(item=>item.path===row.path); return index+step<0 || index+step>=group.length }
// Exposed tree helpers: expandAll / collapseAll only touch folder rows inside the tree; the workspace root stays under the parent's `expanded` prop.
const root = ref<HTMLElement|null>(null)
type RevealOptions = { focus?: boolean; scroll?: boolean; expandWorkspace?: boolean }
function directoryPaths() {
 const dirs=new Set<string>()
 const add=(path:string) => { const parts=path.split('/').filter(Boolean); while(parts.length) { dirs.add(parts.join('/')); parts.pop() } }
 for(const path of files.value.directories || []) add(path)
 for(const note of files.value.notes) { const parts=note.path.split('/'); parts.pop(); add(parts.join('/')) }
 return dirs
}
// Expands every folder; returns how many folders are currently known.
function expandAll() { allCollapsed.value=false;const count=directoryPaths().size; closed.value=new Set();if(!props.expanded)emit('toggle'); return count }
// Collapses every folder so only root-level rows stay visible; returns how many folders are collapsed.
function collapseAll() { allCollapsed.value=true;const dirs=directoryPaths(); closed.value=dirs; return dirs.size }
watch(()=>[...directoryPaths()].sort().join("\0"),()=>{if(allCollapsed.value)closed.value=directoryPaths()},{immediate:true})
// Reveals `path`: expands its ancestor folders, scrolls the row into view and optionally focuses it. Resolves true when the row is on screen.
async function reveal(path:string, options:RevealOptions={}):Promise<boolean> {
 const { focus=false, scroll=true, expandWorkspace=true } = options
 const target=String(path ?? '').trim().replace(/^\/+|\/+$/g,'')
 if(!target) return false
 if(!props.expanded) { if(!expandWorkspace) return false; emit('toggle'); await nextTick() }
 await ensureLoaded()
 const known=directoryPaths().has(target) || files.value.notes.some(note => note.path===target)
 if(!known) return false
 const ancestors=new Set<string>(), parts=target.split('/')
 while(parts.length>1) { parts.pop(); ancestors.add(parts.join('/')) }
 allCollapsed.value=false
 const next=new Set(closed.value); for(const folder of ancestors) next.delete(folder); closed.value=next
 await nextTick()
 const element=root.value ? Array.from(root.value.querySelectorAll<HTMLElement>('[data-entry-path]')).find(node => node.dataset.entryPath===target) || null : null
 if(!element) return false
 if(scroll) element.scrollIntoView({block:'nearest'})
 if(focus) element.focus({preventScroll:true})
 return true
}
defineExpose({ expandAll, collapseAll, reveal })
</script>

<template>
 <div ref="root" class="workspace-tree" :data-workspace-id="workspace.id" @dragover="scrollDuringDrag">
  <div class="workspace-root" @focusin="emit('focusEntry','',true)" @contextmenu.prevent.stop="emit('contextEntry',$event,'',true)" :class="{current:active,'drop-folder':dropTarget===''}" @dragover="overFolder($event,'')" @drop.prevent.stop="dropIntoFolder('')">
   <button class="tree-toggle" :title="(expanded ? '折叠工作区 ' : '展开工作区 ') + workspace.name" :aria-expanded="expanded" @click="emit('toggle')"><ChevronDown v-if="expanded" :size="14"/><ChevronRight v-else :size="14"/></button>
   <button class="workspace-root-name" :class="{selected:active && selected==='dir:.','drop-folder':dropTarget===''}" :title="workspace.path" @click="emit('directory','')" @dragover="overFolder($event,'')" @dragleave="dropTarget=null" @drop.prevent.stop="dropIntoFolder('')"><Folder :size="16"/><span>{{workspace.name}}</span><small v-if="active">当前</small></button>
  </div>
  <div v-if="expanded" class="folder-tree" role="tree" :aria-label="workspace.name + ' 文件'">
   <p v-if="loading" class="folder-hint">正在读取…</p><p v-if="error" class="folder-hint">{{error}}</p>
   <div v-for="row in rows" :key="row.path" class="workspace-tree-row" tabindex="0" role="treeitem" :aria-level="row.depth+1" :aria-expanded="row.note ? undefined : !folderClosed(row.path)" :data-entry-path="row.path" @focusin="emit('focusEntry',row.path,!row.note)" @contextmenu.prevent.stop="emit('contextEntry',$event,row.path,!row.note)" @keydown="treeKeys($event,row)" @keydown.enter.self="row.note ? openRow($event,row.note) : emit('directory',row.path)" :class="{'drop-folder':!row.note && dropTarget===row.path}" :style="{paddingLeft:`${12+row.depth*14}px`}" draggable="true" @dragstart.stop="startDrag($event,row)" @dragend="endDrag" @dragover.prevent="!row.note && overFolder($event,row.path)" @drop.prevent.stop="dropRow(row)">
    <GripVertical :size="11" class="tree-grip" aria-label="拖到文件夹名称可移动文件，拖到抓手可同级排序" @drop.prevent.stop="drop(row.path)"/>
    <template v-if="!row.note"><button class="tree-toggle" :title="(folderClosed(row.path)?'展开 ':'折叠 ')+row.path" :aria-expanded="!folderClosed(row.path)" @click="toggle(row.path)"><ChevronRight v-if="folderClosed(row.path)" :size="13"/><ChevronDown v-else :size="13"/></button><button class="tree-entry" :title="row.path" :class="{selected:active && selected==='dir:'+row.path,'drop-folder':dropTarget===row.path}" @click="emit('directory',row.path)" @dragover="overFolder($event,row.path)" @dragleave="dropTarget=null" @drop.prevent.stop="dropRow(row)"><Folder :size="15"/><span>{{row.name}}</span></button></template>
    <button v-else class="tree-entry tree-file" :class="{selected:active && selected==='file:'+row.note.id,'tree-search-result':!!searchTerm}" :title="row.path" @click="openRow($event,row.note)"><FileText :size="14"/><span v-if="searchTerm" class="tree-file-match"><span class="tree-match-name"><template v-for="(part,index) in highlights(row.name)" :key="index"><mark v-if="part.hit">{{part.text}}</mark><template v-else>{{part.text}}</template></template></span><small v-if="matches.get(row.note.id)?.snippet" class="tree-match-snippet"><template v-for="(part,index) in highlights(matches.get(row.note.id)!.snippet)" :key="index"><mark v-if="part.hit">{{part.text}}</mark><template v-else>{{part.text}}</template></template></small></span><span v-else>{{row.name}}</span></button>
    <div class="tree-order-actions"><button :title="'上移 '+row.name" :disabled="atEdge(row,-1)" @click="move(row.path,-1)"><ArrowUp :size="12"/></button><button :title="'下移 '+row.name" :disabled="atEdge(row,1)" @click="move(row.path,1)"><ArrowDown :size="12"/></button></div>
   </div>
   <p v-if="!loading && !error && !rows.length" class="folder-hint">{{searchTerm?'没有匹配的文件':'此工作区暂无笔记'}}</p>
  </div>
 </div>
</template>
<style scoped>
.workspace-tree-row:hover .tree-entry:not(.selected):not(.drop-folder),
.workspace-root:hover .workspace-root-name:not(.selected):not(.drop-folder){background:color-mix(in srgb,var(--accent-bg) 65%,transparent);color:var(--accent)}
.tree-file.tree-search-result{align-items:flex-start;padding-top:7px;padding-bottom:7px;min-height:34px}.tree-search-result>svg{flex-shrink:0;margin-top:2px}.tree-file .tree-file-match{display:flex;flex-direction:column;gap:4px;flex:1;min-width:0;white-space:normal}.tree-match-name{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;line-height:1.5}.tree-match-snippet{font-size:.72rem;color:var(--muted);line-height:1.65;display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:2;overflow:hidden;overflow-wrap:anywhere;max-height:3.3em}.tree-file mark{background:var(--accent-bg);color:var(--accent);border-radius:2px;padding:0 1px;font-weight:600}
</style>
