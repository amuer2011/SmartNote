import {h,render,type ObjectDirective} from 'vue'
import {Copy,Check,TriangleAlert} from 'lucide-vue-next'

type Block={target:HTMLElement;label:string;timer?:ReturnType<typeof setTimeout>;request:number}
type State={blocks:Map<HTMLButtonElement,Block>;status:HTMLElement;click:(event:MouseEvent)=>void;pointer:(event:PointerEvent)=>void}
const states=new WeakMap<HTMLElement,State>(),icons=new Map<string,Node>()
function icon(name:'copy'|'copied'|'error'){
 let node=icons.get(name)
 if(!node){const host=document.createElement('span');render(h(name==='copied'?Check:name==='error'?TriangleAlert:Copy,{size:15,strokeWidth:1.8,'aria-hidden':'true'}),host);node=host.firstChild!.cloneNode(true);render(null,host);icons.set(name,node)}
 return node.cloneNode(true)
}

/** Copy readable block text; code blocks retain their exact rendered whitespace. */
export function markdownBlockText(root:HTMLElement):string {
 if(root.tagName==='PRE')return root.querySelector('code')?.textContent||root.textContent||''
 const text=(node:Node):string=>{
  if(node.nodeType===Node.TEXT_NODE)return node.textContent||''
  if(!(node instanceof HTMLElement)||node.matches('[data-markdown-copy-ui],.admonition-icon,.admonition-chevron'))return ''
  const tag=node.tagName
  if(tag==='BR')return '\n'
  if(tag==='IMG')return node.getAttribute('alt')||''
  if(tag==='PRE')return '\n'+markdownBlockText(node)+'\n'
  const value=Array.from(node.childNodes,text).join('')
  if(tag==='LI'){
   const parent=node.parentElement,number=parent?.tagName==='OL'?(Number(parent.getAttribute('start')||1)+Array.from(parent.children).indexOf(node))+'. ':'- '
   return '\n'+number+value.trim()+'\n'
  }
  if(tag==='TD'||tag==='TH')return value+(node.nextElementSibling?'\t':'')
  if(/^(P|H[1-6]|DIV|BLOCKQUOTE|UL|OL|TABLE|TR|ASIDE|DETAILS|SUMMARY)$/.test(tag))return '\n'+value+'\n'
  return value
 }
 return Array.from(root.childNodes,text).join('').replace(/\n{3,}/g,'\n\n').trim()
}

function feedback(button:HTMLButtonElement,block:Block,state:'copy'|'copied'|'error'){
 button.dataset.copyState=state;button.replaceChildren(icon(state));button.title=state==='copied'?'已复制':state==='error'?'复制失败，请重试':block.label
 button.setAttribute('aria-label',state==='copy'?block.label:state==='copied'?'已复制'+block.label.slice(2):'复制失败，请重试')
}
function enhance(root:HTMLElement){
 const state=states.get(root)!
 for(const [button,block] of state.blocks){if(!root.contains(button)){clearTimeout(block.timer);state.blocks.delete(button)}}
 const installed=new Set(Array.from(state.blocks.values(),block=>block.target))
 for(const target of root.querySelectorAll<HTMLElement>('.admonition-body,pre,blockquote')){
  if(installed.has(target))continue
  let surface=target
  if(target.tagName==='PRE'){surface=document.createElement('div');surface.className='markdown-code-copy';target.before(surface);surface.append(target)}
  surface.classList.add('markdown-copy-surface')
  const label=target.tagName==='PRE'?'复制代码':target.tagName==='BLOCKQUOTE'?'复制引用内容':'复制提示块内容'
  const button=document.createElement('button'),block:Block={target,label,request:0}
  button.type='button';button.className='markdown-copy-button';button.dataset.markdownCopyUi='';feedback(button,block,'copy');surface.append(button);state.blocks.set(button,block)
 }
 if(!root.contains(state.status))root.append(state.status)
}

export const vBlockCopy:ObjectDirective<HTMLElement>={
 mounted(root){
  const status=document.createElement('span');status.className='markdown-copy-status';status.dataset.markdownCopyUi='';status.setAttribute('role','status');status.setAttribute('aria-live','polite')
  const state:State={blocks:new Map(),status,
   pointer(event){if(event.target instanceof Element&&event.target.closest('.markdown-copy-button')){event.preventDefault();event.stopPropagation()}},
   async click(event){
    const button=event.target instanceof Element?event.target.closest<HTMLButtonElement>('.markdown-copy-button'):null,block=button&&state.blocks.get(button)
    if(!button||!block)return
    event.preventDefault();event.stopPropagation();clearTimeout(block.timer);const request=++block.request
    const value=markdownBlockText(block.target);status.textContent=''
    try{await navigator.clipboard.writeText(value);if(states.get(root)!==state||!root.contains(button)||request!==block.request)return;feedback(button,block,'copied');status.textContent='已复制'}
    catch{if(states.get(root)!==state||!root.contains(button)||request!==block.request)return;feedback(button,block,'error');status.textContent='复制失败，请重试'}
    block.timer=setTimeout(()=>{if(root.contains(button))feedback(button,block,'copy')},1800)
   },
  }
  states.set(root,state);root.addEventListener('click',state.click,true);root.addEventListener('pointerdown',state.pointer,true);enhance(root)
 },
 updated:enhance,
 beforeUnmount(root){const state=states.get(root);if(!state)return;root.removeEventListener('click',state.click,true);root.removeEventListener('pointerdown',state.pointer,true);for(const block of state.blocks.values())clearTimeout(block.timer);states.delete(root)},
}
