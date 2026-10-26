import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api, desktop, type Note, type Snapshot, type ArchiveResult, type FolderImportInput, type FolderImportResult } from '../services/api'

export const useWorkspace = defineStore('workspace', () => {
 const data = ref<Snapshot>({ settings: { workspaces: [], activeId: '', picGoUrl: '', theme: 'light' }, notes: [], tasks: [], attachments: [] })
 const active = ref<Note | null>(null), content = ref(''), error = ref(''), status = ref('已保存到本地'), conflict = ref(false), saving = ref(false)
 const workspace = computed(() => data.value.settings.workspaces.find(w => w.id === data.value.settings.activeId))
 const dirty = computed(() => active.value !== null && active.value.content !== content.value)
 const switching=ref(false)
 let refreshSequence=0
 const autoArchiveResult=ref<ArchiveResult|null>(null)
 const automaticSkips=new Map<string,string>()
 function suppressAutomaticArchive(note:Note){
  const id=workspace.value?.id;if(!id)return
  if(automaticSkips.size>=256)automaticSkips.clear()
  automaticSkips.set(`${id}:${note.id}`,note.hash);clearTimeout(autoTimer)
 }
 let autoTimer:ReturnType<typeof setTimeout>|undefined
 let autoRunning=false
 let autoFlight:Promise<void>|null=null
 function scheduleArchive(note:Note) {
  clearTimeout(autoTimer)
  const workspaceID=workspace.value?.id
  if(!workspaceID || note.automationIgnored || data.value.notes.find(n=>n.id===note.id)?.automationIgnored)return
  if(automaticSkips.get(`${workspaceID}:${note.id}`)===note.hash)return
  autoTimer=setTimeout(async()=>{
   if(autoRunning || saving.value || dirty.value || workspace.value?.id!==workspaceID || active.value?.id!==note.id || active.value.hash!==note.hash)return
   autoRunning=true
   autoFlight=(async()=>{
   try {
    const result=await api.AutoArchive(workspaceID,note.id,note.hash)
    if(!result || workspace.value?.id!==workspaceID)return
    if(active.value?.id===note.id && active.value.hash===note.hash){const edited=dirty.value;active.value=result.note;if(!edited)content.value=result.note.content}
    if(result.operationId) autoArchiveResult.value=result;await refresh()
   }catch(e){if(workspace.value?.id===workspaceID && !String(e).includes('CONFLICT'))status.value='已保存；自动归档未完成：'+String(e)}
   finally{autoRunning=false;autoFlight=null}
   })();await autoFlight
  },4000)
 }
 let timer: ReturnType<typeof setTimeout> | undefined
 let inFlight: Promise<void> | null = null
 const draftKey = () => `smartnote:draft:${data.value.settings.activeId}:${active.value?.id}`
 function report(e: unknown) { const msg = String(e); if (!msg.includes('已取消')) error.value = msg }
 async function refresh() {
  if (!desktop || switching.value) return
  const request=++refreshSequence, workspaceID=workspace.value?.id
  try {
   const snapshot = await api.Snapshot()
   if(request!==refreshSequence || switching.value || workspace.value?.id!==workspaceID)return
   data.value = snapshot
   if (active.value) {
    const fresh = snapshot.notes.find(n => n.id === active.value?.id)
    if (!fresh) { if (dirty.value) { conflict.value = true; status.value = '原文件已移除，草稿已保留' } else { active.value = null; content.value = '' } }
    else if (fresh.hash !== active.value.hash) {
     if (dirty.value || saving.value) { conflict.value = true; status.value = '检测到外部修改，请比较后合并' }
     else { active.value = fresh; content.value = fresh.content }
    } else { active.value = {...active.value,automationIgnored:fresh.automationIgnored} }
   }
  } catch(e) { if(request===refreshSequence && !switching.value)report(e) }
 }
 async function save() {
  clearTimeout(timer)
  if(autoFlight) await autoFlight
  if (inFlight) { await inFlight; if (dirty.value && !conflict.value) return save(); return }
  if (!active.value || !dirty.value || conflict.value) return
  const note = active.value, text = content.value, key = draftKey(); saving.value = true; status.value = '正在保存…'
  inFlight = (async () => {
   try {
    const saved = await api.SaveNote({ id: note.id, path: note.path, content: text, hash: note.hash })
    if (active.value?.id === note.id) active.value = saved
    const idx = data.value.notes.findIndex(n => n.id === note.id); if (idx >= 0) data.value.notes[idx] = saved
    if (content.value === text) { content.value = saved.content; localStorage.removeItem(key) }
    await refresh()
    status.value = '已保存到本地';scheduleArchive(saved)
   } catch(e) { if (String(e).includes('CONFLICT')) conflict.value = true; status.value = '保存失败，草稿已保留'; report(e) }
   finally { saving.value = false; inFlight = null }
  })()
  await inFlight
  if (dirty.value && !conflict.value && status.value === '已保存到本地') scheduleSave()
 }
 function scheduleSave() { clearTimeout(timer); timer = setTimeout(() => void save(), 800) }
 function edit(text: string) { clearTimeout(autoTimer); content.value = text; if (active.value) { localStorage.setItem(draftKey(), JSON.stringify({ content: text, hash: active.value.hash })); status.value = '未保存'; scheduleSave() } }
 async function select(note: Note) {
  if(switching.value)return false
  const workspaceID=workspace.value?.id
  await save();if(switching.value || workspaceID!==workspace.value?.id)return false; if (dirty.value && active.value?.id !== note.id) { error.value = '当前笔记仍有未保存修改，请先处理保存冲突。'; return false }
  // Saving may wait for automatic archiving, which can change this target's path/hash.
  note = data.value.notes.find(current => current.id === note.id) || note
  active.value = note; content.value = note.content; conflict.value = false
  const draft = localStorage.getItem(draftKey()); if (draft) { try { const d = JSON.parse(draft); if (d.content !== note.content) { content.value = d.content; conflict.value = d.hash !== note.hash; status.value = conflict.value ? '已恢复草稿，磁盘文件有变化' : '已恢复未保存草稿'; if (!conflict.value) scheduleSave() } } catch { error.value = '草稿读取失败，请检查浏览器存储。' } }
  if(!dirty.value) scheduleArchive(note)
  return true
 }
 // Publish workspace settings and their data together. Discard snapshots started
 // before a switch, even when their bridge response arrives much later.
 async function changeWorkspace(open:()=>Promise<string>) {
  if(switching.value)return false
  switching.value=true;refreshSequence++;clearTimeout(autoTimer)
  const previous=workspace.value?.id
  let opened=false
  try {
   await save()
   if(dirty.value){error.value='请先处理当前笔记的未保存修改。';return false}
   const id=await open();opened=true
   const snapshot=await api.Snapshot()
   if(snapshot.settings.activeId!==id)throw new Error('工作区切换未完成，请重试。')
   active.value=null;content.value='';conflict.value=false;autoArchiveResult.value=null
   data.value=snapshot;status.value='已保存到本地';error.value=''
   return true
  } catch(e) {
   // If loading failed after opening the directory, keep the backend aligned
   // with the workspace still displayed by the app.
   if(opened && previous){try{await api.OpenWorkspace(previous)}catch(rollback){report(rollback)}}
   throw e
  } finally {refreshSequence++;switching.value=false}
 }
 async function switchWorkspace(id:string){if(id===workspace.value?.id)return true;return changeWorkspace(async()=>{await api.OpenWorkspace(id);return id})}
 async function importZip(){return changeWorkspace(async()=>{const selected=await api.Import();return selected.id})}
 async function importFolder(input:FolderImportInput){
  let result:FolderImportResult|undefined
  const changed=await changeWorkspace(async()=>{result=await api.ImportFolder(input);return result.workspace.id})
  return changed?result:undefined
 }
 async function chooseWorkspace(){return changeWorkspace(async()=>{const selected=await api.ChooseWorkspace();return selected.id})}
 async function reload() { if (!active.value || switching.value) return; const workspaceID=workspace.value?.id,noteID=active.value.id; const latest = await api.Snapshot(); if(workspaceID!==workspace.value?.id || active.value?.id!==noteID || latest.settings.activeId!==workspaceID)return; const fresh = latest.notes.find(n => n.id === active.value?.id); if (!fresh) { error.value = '磁盘文件已删除，请通过复制草稿恢复内容。'; return } active.value = fresh; content.value = fresh.content; conflict.value = false; localStorage.removeItem(draftKey()); status.value = '已载入磁盘版本'; await refresh() }
 async function merge(text: string) { if (!active.value || switching.value) return; const workspaceID=workspace.value?.id,noteID=active.value.id; const latest = await api.Snapshot(); if(workspaceID!==workspace.value?.id || active.value?.id!==noteID || latest.settings.activeId!==workspaceID)return; const fresh = latest.notes.find(n => n.id === active.value?.id); if (!fresh) { error.value = '磁盘文件已删除，请新建笔记恢复草稿。'; return } active.value = fresh; conflict.value = false; edit(text); await save() }
 return { suppressAutomaticArchive, autoArchiveResult, data, active, content, workspace, dirty, error, status, conflict, saving, refresh, save, edit, select, switching, switchWorkspace, chooseWorkspace, importZip, importFolder, report, reload, merge }
})
