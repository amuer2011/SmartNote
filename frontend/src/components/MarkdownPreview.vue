<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'
import {admonitions} from '../services/admonitions'
import {vBlockCopy} from '../services/markdown-copy'
import '../markdown-copy.css'
import {textMatches} from '../services/text-search'
import { api } from '../services/api'
const props = defineProps<{ content: string }>()
const emit = defineEmits<{ link: [target: string]; image: [src: string,attachmentId?:string] }>()
const host=ref<HTMLElement>()
let findTerm='',findIndex=0,findRegex=false,findCase=false
const resources = ref<Record<string, string>>({})
const md = new MarkdownIt({ html: false, linkify: true, breaks: true }).use(admonitions)
const defaultValidate = md.validateLink.bind(md)
md.validateLink = url => url.startsWith('smartnote-asset://') || defaultValidate(url)
md.inline.ruler.before('link', 'smartnote-wiki', (state, silent) => {
 if (state.src.slice(state.pos,state.pos+2)!=='[[')return false
 const end=state.src.indexOf(']]',state.pos+2);if(end<0)return false
 const [target='',label]=state.src.slice(state.pos+2,end).split('|',2);if(!target || target.includes('\n'))return false
 if(!silent){const open=state.push('link_open','a',1);open.attrSet('href',`smartnote-note://${encodeURIComponent(target)}`);state.push('text','',0).content=label || target;state.push('link_close','a',-1)}
 state.pos=end+2;return true
})
const defaultImage=md.renderer.rules.image!
md.renderer.rules.image=(tokens,idx,options,env,self)=>{
 const token=tokens[idx]!,src=token.attrGet('src') || ''
 if(src.startsWith('smartnote-asset://')){token.attrSet('data-attachment-id',src.slice(18));const resource=resources.value[src.slice(18)];if(resource)token.attrSet('src',resource)}
 return defaultImage(tokens,idx,options,env,self)
}
const defaultHeading=md.renderer.rules.heading_open
md.renderer.rules.heading_open=(tokens,index,options,env,self)=>{const map=tokens[index].map;if(map)tokens[index].attrSet('data-source-line',String(map[0]+(env?.lineOffset||0)));return defaultHeading?defaultHeading(tokens,index,options,env,self):self.renderToken(tokens,index,options)}
for(const name of ['paragraph_open','blockquote_open','bullet_list_open','ordered_list_open','list_item_open']){
 const original=md.renderer.rules[name]
 md.renderer.rules[name]=(tokens,index,options,env,self)=>{const map=tokens[index].map;if(map)tokens[index].attrSet('data-source-line',String(map[0]+(env?.lineOffset||0)));return original?original(tokens,index,options,env,self):self.renderToken(tokens,index,options)}
}
const rendered = computed(() => {
 const text = props.content.replace(/^---\r?\n[\s\S]*?\r?\n---(?:\r?\n|$)/, '')
 const lineOffset=props.content.slice(0,props.content.length-text.length).split('\n').length-1
 return DOMPurify.sanitize(md.render(text,{lineOffset}), { ADD_URI_SAFE_ATTR: [], ALLOWED_URI_REGEXP: /^(?:(?:https?|mailto|smartnote-asset|smartnote-note):|data:image\/(?:png|jpeg|gif|webp|avif|bmp);base64,|[^a-z]|[a-z+.-]+(?:[^a-z+.-:]|$))/i })
})
watch(() => props.content, async text => { const ids = [...text.matchAll(/smartnote-asset:\/\/([a-zA-Z0-9_-]+)/g)].map(m => m[1]!); for (const id of ids) { if (!resources.value[id]) { try { const data = await api.AttachmentData(id); if (/^data:image\/(png|jpeg|gif|webp|avif|bmp);/.test(data)) resources.value[id] = data } catch { /* Keep the unresolved reference visible. */ } } } }, { immediate: true })
function click(e: MouseEvent) { const target = e.target as HTMLElement; if (target.tagName === 'IMG') { e.preventDefault(); emit('image', (target as HTMLImageElement).src,(target as HTMLElement).dataset.attachmentId); return } const link = target.closest('a'); if (link) { e.preventDefault(); emit('link', link.getAttribute('href') ?? '') } }
function scrollElement(element:HTMLElement){for(let parent=element.parentElement;parent&&parent!==host.value;parent=parent.parentElement){if(parent instanceof HTMLDetailsElement)parent.open=true};const pane=host.value?.closest<HTMLElement>('.preview-pane');if(pane)pane.scrollTop+=element.getBoundingClientRect().top-pane.getBoundingClientRect().top-pane.clientHeight/2}
function scrollToLine(line:number){
 const blocks=[...(host.value?.querySelectorAll<HTMLElement>('[data-source-line]')||[])]
 const exact=blocks.find(block=>Number(block.dataset.sourceLine)===line)
 const nearest=blocks.filter(block=>Number(block.dataset.sourceLine)<=line).sort((a,b)=>Number(b.dataset.sourceLine)-Number(a.dataset.sourceLine))[0]
 if(exact||nearest)scrollElement((exact||nearest)!)
}
function findMatches(term:string,index=0,regex=false,caseSensitive=false){
 findTerm=term;findIndex=index;findRegex=regex;findCase=caseSensitive
 const root=host.value;if(!root)return 0
 root.querySelectorAll('mark[data-note-find]').forEach(mark=>mark.replaceWith(document.createTextNode(mark.textContent||'')));root.normalize()
 if(!term)return 0
 const walker=document.createTreeWalker(root,NodeFilter.SHOW_TEXT),nodes:{node:Text;start:number;end:number}[]=[]
 let text='',node:Node|null
 while((node=walker.nextNode())){if(node.parentElement?.closest('[data-markdown-copy-ui]'))continue;const value=node.textContent||'';nodes.push({node:node as Text,start:text.length,end:text.length+value.length});text+=value}
 const matches=textMatches(text,term,regex,caseSensitive)
 const selected=matches.length?((index%matches.length)+matches.length)%matches.length:0
 for(const entry of nodes){const hits=matches.map((match,i)=>({start:Math.max(match.start,entry.start)-entry.start,end:Math.min(match.end,entry.end)-entry.start,i})).filter(hit=>hit.start<hit.end)
  if(!hits.length)continue
  const value=entry.node.data,fragment=document.createDocumentFragment();let offset=0
  for(const hit of hits){fragment.append(value.slice(offset,hit.start));const mark=document.createElement('mark');mark.dataset.noteFind=String(hit.i);if(hit.i===selected)mark.className='note-find-current';mark.textContent=value.slice(hit.start,hit.end);fragment.append(mark);offset=hit.end}
  fragment.append(value.slice(offset));entry.node.replaceWith(fragment)
 }
 const active=root.querySelector<HTMLElement>('.note-find-current');if(active)scrollElement(active)
 return matches.length
}
watch(rendered,async()=>{await nextTick();if(findTerm)findMatches(findTerm,findIndex,findRegex,findCase)})
defineExpose({scrollToLine,findMatches})

</script>
<template><article ref="host" v-block-copy class="markdown-preview" v-html="rendered" @click="click" /></template>
