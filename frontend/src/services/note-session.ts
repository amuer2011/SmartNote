import type {Note} from './api'

export type OpenNoteSession=Pick<Note,'id'|'path'>
const key=(directory:string,workspaceId:string)=>'smartnote:last-open-note:'+JSON.stringify([directory,workspaceId])

export function readNoteSession(directory:string,workspaceId:string):OpenNoteSession|undefined{
 try{
  const raw=localStorage.getItem(key(directory,workspaceId))
  if(!raw)return
  const value=JSON.parse(raw)
  if(value&&typeof value.id==='string'&&value.id&&typeof value.path==='string'&&value.path)return {id:value.id,path:value.path}
 }catch{/* An unavailable browser store must not prevent opening notes. */}
}
export function rememberNoteSession(directory:string,workspaceId:string,note:OpenNoteSession){
 try{localStorage.setItem(key(directory,workspaceId),JSON.stringify({id:note.id,path:note.path}))}catch{/* Reading and editing remain available without session storage. */}
}
export function clearNoteSession(directory:string,workspaceId:string){
 try{localStorage.removeItem(key(directory,workspaceId))}catch{/* Session state is optional. */}
}
