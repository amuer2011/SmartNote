import type {Note} from './api'

export interface NavigationMatch {snippet:string;line:number}
export function navigationMatch(note:Note,query:string):NavigationMatch|undefined {
 const term=query.trim();if(!term)return
 const pattern=new RegExp(term.replace(/[.*+?^${}()|[\]\\]/g,'\\$&'),'iu')
 const metadata=note.content.match(/^(?:\uFEFF)?---\r?\n[\s\S]*?\r?\n---(?:\r?\n|$)/)?.[0]||''
 const body=note.content.slice(metadata.length),match=pattern.exec(body)
 if(!match)return pattern.test(note.path.split('/').pop()||'')?{snippet:'',line:0}:undefined
 const prefix=Array.from(body.slice(0,match.index)),hit=Array.from(match[0]),suffix=Array.from(body.slice(match.index+match[0].length))
 // Put the hit near the beginning so two-line tree rows keep it visible even
 // for long keywords or deeply nested files in a narrow sidebar.
 const prefixLimit=hit.length>12?0:6
 const before=prefixLimit?prefix.slice(-prefixLimit).join(''):'',after=suffix.slice(0,Math.max(12,56-Math.min(hit.length,32)-prefixLimit)).join('')
 const excerpt=hit.length>32?hit.slice(0,32).join('')+'…':match[0]
 return {snippet:((prefix.length>prefixLimit?'…':'')+before+excerpt+after+(suffix.length>Array.from(after).length?'…':'')).replace(/\s+/g,' ').trim(),line:note.content.slice(0,metadata.length+match.index).split('\n').length-1}
}
export function navigationHighlight(text:string,query:string):{text:string;hit:boolean}[]{
 const term=query.trim();if(!term)return [{text,hit:false}]
 const pattern=new RegExp(term.replace(/[.*+?^${}()|[\]\\]/g,'\\$&'),'giu'),parts:{text:string;hit:boolean}[]=[]
 let from=0
 for(const match of text.matchAll(pattern)){if(match.index!>from)parts.push({text:text.slice(from,match.index),hit:false});parts.push({text:match[0],hit:true});from=match.index!+match[0].length}
 if(from<text.length)parts.push({text:text.slice(from),hit:false})
 return parts
}
