import {autocompletion,acceptCompletion,currentCompletions,pickedCompletion,type Completion,type CompletionContext,type CompletionResult} from '@codemirror/autocomplete'
import {ensureSyntaxTree,syntaxTree} from '@codemirror/language'
import {EditorState,Prec} from '@codemirror/state'
import {keymap} from '@codemirror/view'
import {h,render,type Component} from 'vue'
import {Pencil,NotebookText,Info,Flame,Check,CircleHelp,TriangleAlert,X,Bug,List,Quote} from 'lucide-vue-next'
import {admonitionTypes} from './admonitions'

type AdmonitionCompletion=Completion & {admonitionKind:string}
const isAdmonition=(option:Completion):option is AdmonitionCompletion=>'admonitionKind' in option
const icons:Record<string,Component>={note:Pencil,abstract:NotebookText,info:Info,todo:Info,tip:Flame,success:Check,question:CircleHelp,warning:TriangleAlert,failure:X,danger:TriangleAlert,bug:Bug,example:List,quote:Quote}
const iconCache=new Map<string,Node>()
function completionIcon(option:Completion){
 if(!isAdmonition(option))return null
 let icon=iconCache.get(option.admonitionKind)
 if(!icon){
  const host=document.createElement('span');render(h(icons[option.admonitionKind],{size:16,strokeWidth:2,'aria-hidden':'true'}),host)
  icon=host.firstChild!.cloneNode(true);render(null,host);iconCache.set(option.admonitionKind,icon)
 }
 const span=document.createElement('span');span.className='cm-admonition-icon';span.setAttribute('aria-hidden','true');span.append(icon.cloneNode(true));return span
}

export function admonitionCompletion(context:CompletionContext):CompletionResult|null {
 if(context.state.selection.main.from!==context.state.selection.main.to)return null
 const line=context.state.doc.lineAt(context.pos),prefix=line.text.slice(0,context.pos-line.from)
 const fence=/^ {0,3}(?:>[ \t]*)*(`{3,}|~{3,})[ \t]*ad-([a-z]*)$/i.exec(prefix)
 const callout=/^ {0,3}(?:>[ \t]*)+\[!([a-z]*)$/i.exec(prefix)
 if(!fence&&!callout)return null
 const tree=ensureSyntaxTree(context.state,context.pos,20)||syntaxTree(context.state)
 let fencedFrom=-1
 for(let node=tree.resolveInner(context.pos,-1);node;node=node.parent!){
  if(node.name==='CodeBlock'||node.name==='InlineCode')return null
  if(node.name==='FencedCode'){fencedFrom=node.from;break}
 }
 // A fence-looking line inside another code block is still code, not an opener.
 if((fence&&fencedFrom!==line.from+prefix.indexOf(fence[1]))||(callout&&fencedFrom>=0))return null
 const typed=(fence?fence[2]:callout![1]).toLowerCase(),from=context.pos-typed.length
 const to=context.pos+(/^[a-z]*/i.exec(line.text.slice(context.pos-line.from))?.[0].length||0)
 const options:AdmonitionCompletion[]=admonitionTypes.filter(type=>type.name.startsWith(typed)).map(type=>({
  label:type.name,admonitionKind:type.kind,
  apply(view,completion,start,end){
   const closed=!!callout&&view.state.sliceDoc(end,end+1)===']',text=completion.label+(callout&&!closed?']':'')
   view.dispatch({changes:{from:start,to:end,insert:text},selection:{anchor:start+text.length+(closed?1:0)},annotations:pickedCompletion.of(completion),userEvent:'input.complete'})
  },
 }))
 return options.length?{from,to,options,filter:false}:null
}

export function admonitionCompletionExtension(){return [
 EditorState.languageData.of(()=>[{autocomplete:admonitionCompletion}]),
 autocompletion({
  activateOnTypingDelay:80,
  interactionDelay:0,
  tooltipClass:state=>currentCompletions(state).some(isAdmonition)?'cm-admonition-completion':'',
  optionClass:option=>isAdmonition(option)?`cm-admonition-option cm-admonition-${option.admonitionKind}`:'',
  addToOptions:[{position:90,render:completionIcon}],
 }),
 Prec.highest(keymap.of([{key:'Tab',run:view=>currentCompletions(view.state).some(isAdmonition)&&acceptCompletion(view)}])),
]}
