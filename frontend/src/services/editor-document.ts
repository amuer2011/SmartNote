export function editorDocument(content:string,title='',showMetadata=false){
 if(showMetadata)return {prefix:'',body:content,lineOffset:0}
 const yaml=/^(?:\uFEFF)?---\r?\n[\s\S]*?\r?\n---(?:\r?\n|$)/.exec(content)
 if(!yaml)return {prefix:'',body:content,lineOffset:0}
 let end=yaml[0].length
 const rest=content.slice(end),heading=/^(?:\r?\n)*# ([^\r\n]+)\r?\n/.exec(rest)
 if(heading&&title&&heading[1]===title)end+=heading[0].length
 // Preserve at least the final blank body line in an otherwise new note.
 let prefix=content.slice(0,end)
 if(end===content.length&&!prefix.endsWith('\n'))prefix+='\n'
 return {prefix,body:content.slice(end),lineOffset:prefix.split('\n').length-1}
}
export function noteTimestamp(value:string){
 const date=new Date(value);if(!Number.isFinite(date.getTime()))return '—'
 const p=(n:number)=>String(n).padStart(2,'0')
 return `${date.getFullYear()}-${p(date.getMonth()+1)}-${p(date.getDate())} ${p(date.getHours())}:${p(date.getMinutes())}:${p(date.getSeconds())}`
}
