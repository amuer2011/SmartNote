import type MarkdownIt from 'markdown-it'

const kinds: Record<string, [string, string]> = {
 note:['笔记','✎'], abstract:['摘要','≡'], info:['信息','i'], todo:['待办','☐'],
 tip:['提示','✦'], success:['成功','✓'], question:['问题','?'], warning:['警告','!'],
 failure:['失败','×'], danger:['危险','!'], bug:['问题排查','⚑'], example:['示例','≡'], quote:['引用','❝'],
}
const aliases: Record<string,string> = {seealso:'note',summary:'abstract',tldr:'abstract',hint:'tip',important:'tip',check:'success',done:'success',help:'question',faq:'question',caution:'warning',attention:'warning',fail:'failure',missing:'failure',error:'danger',cite:'quote'}
const kindOf=(name:string)=>{const key=name.toLowerCase();return aliases[key] || (kinds[key]?key:'note')}
// The renderer and editor share the supported types and aliases.
export const admonitionTypes = ['note','seealso','abstract','summary','tldr','info','todo','tip','hint','important','success','check','done','question','help','faq','warning','caution','attention','failure','fail','missing','danger','error','bug','example','quote','cite'].map(name=>({name,kind:kindOf(name),title:kinds[kindOf(name)][0]}))

/** Native syntax compatibility; no Obsidian runtime or executable HTML is loaded. */
export function admonitions(md:MarkdownIt) {
 const title=(kind:string,text:string,fold:boolean,env:Record<string,unknown>)=>{
  const tag=fold?'summary':'div',label=md.renderInline(text,env)
  return `<${tag} class="admonition-title"><span class="admonition-icon" aria-hidden="true">${kinds[kind][1]}</span><span>${label}</span>${fold?'<span class="admonition-chevron" aria-hidden="true">›</span>':''}</${tag}>\n`
 }
 const fence=md.renderer.rules.fence!
 md.renderer.rules.fence=(tokens,index,options,env,self)=>{
  const token=tokens[index],match=/^ad-([\w-]+)(?:\s+(.*))?$/.exec(token.info.trim())
  if(!match || (env?.admonitionDepth||0)>=16)return fence(tokens,index,options,env,self)
  const kind=kindOf(match[1]),lines=token.content.split('\n');let consumed=0,label=match[2]||kinds[kind][0],collapse=''
  while(consumed<lines.length){
   const meta=/^(title|collapse):\s*(.*)$/i.exec(lines[consumed]);if(!meta)break
   if(meta[1].toLowerCase()==='title')label=meta[2];else collapse=meta[2].trim().toLowerCase()
   consumed++
  }
  const fold=['open','closed','true','false','+','-'].includes(collapse),open=['open','false','+'].includes(collapse)
  const tag=fold?'details':'aside',line=(token.map?.[0]||0)+(env?.lineOffset||0)
  const bodyEnv={...env,lineOffset:line+1+consumed,admonitionDepth:(env?.admonitionDepth||0)+1}
  return `<${tag} class="admonition admonition-${kind}" data-source-line="${line}"${fold&&open?' open':''}>\n${title(kind,label,fold,env)}<div class="admonition-body">\n${md.render(lines.slice(consumed).join('\n'),bodyEnv)}</div></${tag}>\n`
 }
 md.core.ruler.after('block','smartnote-callouts',state=>{
  const tokens=state.tokens
  for(let i=0;i<tokens.length;i++){
   const start=tokens[i],paragraph=tokens[i+1],inline=tokens[i+2]
   if(start.type!=='blockquote_open'||paragraph?.type!=='paragraph_open'||inline?.type!=='inline')continue
   const match=/^\[!([\w-]+)\]([+-]?)(?:[ \t]+([^\n]*))?(?:\n|$)/.exec(inline.content)
   if(!match)continue
   let end=i+1,depth=1
   for(;end<tokens.length;end++){if(tokens[end].type==='blockquote_open')depth++;if(tokens[end].type==='blockquote_close'&&!--depth)break}
   if(end>=tokens.length)continue
   const kind=kindOf(match[1]),fold=!!match[2]
   start.type='admonition_open';start.tag=fold?'details':'aside';start.attrSet('class',`admonition admonition-${kind}`)
   start.attrSet('data-source-line',String((start.map?.[0]||0)+(state.env?.lineOffset||0)))
   if(match[2]==='+')start.attrSet('open','')
   tokens[end].type='admonition_close';tokens[end].tag=start.tag
   const heading=new state.Token('admonition_title','',0);heading.meta={kind,label:match[3]||kinds[kind][0],fold}
   const body=new state.Token('admonition_body_open','div',1);body.attrSet('class','admonition-body')
   const close=new state.Token('admonition_body_close','div',-1)
   tokens.splice(end,0,close)
   inline.content=inline.content.slice(match[0].length)
   if(inline.content){if(inline.map)inline.map[0]++;if(paragraph.map)paragraph.map[0]++;tokens.splice(i+1,0,heading,body)}
   else tokens.splice(i+1,3,heading,body)
  }
 })
 md.renderer.rules.admonition_title=(tokens,index,_options,env)=>{const {kind,label,fold}=tokens[index].meta;return title(kind,label,fold,env)}
}
