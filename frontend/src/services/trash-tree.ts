import type {TrashEntry} from './api'
export interface TrashRow {id:string;path:string;name:string;directory:boolean;entry?:TrashEntry}
export interface TrashPreview {entry:TrashEntry;path:string;content:string;loading:boolean;error:string}
export const originalTrashPath=(entry:TrashEntry)=>entry.path.split('/').slice(1).join('/')
export const trashParent=(path:string)=>path.includes('/')?path.slice(0,path.lastIndexOf('/')):''
export function trashRows(entries:TrashEntry[],source:string,directory:string):TrashRow[]{
 const dirs=new Set<string>(),files:TrashRow[]=[]
 for(const entry of entries){
  if(entry.source!==source)continue
  const path=originalTrashPath(entry);if(!path)continue
  let folder=entry.directory?path:trashParent(path)
  while(folder){dirs.add(folder);folder=trashParent(folder)}
  if(!entry.directory&&trashParent(path)===directory)files.push({id:entry.source+':'+entry.path,path,name:entry.name,directory:false,entry})
 }
 return [...[...dirs].filter(path=>trashParent(path)===directory).map(path=>({id:'dir:'+source+':'+path,path,name:path.split('/').pop()!,directory:true})),...files].sort((a,b)=>Number(b.directory)-Number(a.directory)||a.name.localeCompare(b.name,'zh-CN')||a.id.localeCompare(b.id))
}
export function trashTargets(entries:TrashEntry[],source:string,row:TrashRow):TrashEntry[]{
 if(!row.directory)return row.entry?[row.entry]:[]
 const candidates=entries.filter(e=>e.source===source&&(originalTrashPath(e)===row.path||originalTrashPath(e).startsWith(row.path+'/'))).sort((a,b)=>a.path.split('/').length-b.path.split('/').length)
 const roots:TrashEntry[]=[]
 for(const entry of candidates)if(!roots.some(root=>root.directory&&entry.path.startsWith(root.path+'/')))roots.push(entry)
 return roots
}
