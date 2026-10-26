import type { Note } from './api'

export const noteDragType = 'application/x-smartnote-note'
export interface DraggedNote { workspaceId:string; path:string; title:string }
export function startNoteDrag(event:DragEvent,workspaceId:string,note:Note) {
 const data=event.dataTransfer;if(!data) return
 data.effectAllowed='copyMove'
 data.setData(noteDragType,JSON.stringify({workspaceId,path:note.path,title:note.title}))
 data.setData('text/plain',note.path)
}
export function readNoteDrag(data:DataTransfer):DraggedNote|null {
 try {
  const value=JSON.parse(data.getData(noteDragType))
  if(typeof value.workspaceId==='string' && typeof value.path==='string' && typeof value.title==='string' && value.path.endsWith('.md') && !value.path.split('/').some((part:string)=>!part || part==='.' || part==='..')) return value
 } catch { /* Ignore unrelated drag payloads. */ }
 return null
}
export function noteReference(sourcePath:string,note:DraggedNote):string {
 const from=sourcePath.split('/');from.pop();const to=note.path.split('/')
 while(from.length && to.length && from[0]===to[0]) {from.shift();to.shift()}
 const path=[...from.map(()=>'..'),...to].map(part=>encodeURIComponent(part).replace(/[!'()*]/g,char=>'%'+char.charCodeAt(0).toString(16).toUpperCase())).join('/')
 const label=note.title.replace(/\r?\n/g,' ').replace(/[\\[\]`*<>!]/g,'\\$&')
 return '['+label+']('+path+')'
}
