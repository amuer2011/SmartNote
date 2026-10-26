<script setup lang="ts">
import { noteDragType, readNoteDrag, noteReference } from '../services/note-drag'
import {clipboardFileImports} from '../services/clipboard-files'
import {api} from '../services/api'
import { onMounted, onBeforeUnmount, watch, ref } from 'vue'
import { EditorView, keymap } from '@codemirror/view'
import {editorDocument} from '../services/editor-document'
import {admonitionCompletionExtension} from '../services/admonition-completion'
import {fenceCompletionExtension} from '../services/fence-completion'
import { EditorState, Transaction } from '@codemirror/state'
import { basicSetup } from 'codemirror'
import { markdown } from '@codemirror/lang-markdown'
import { undo, redo, selectAll } from '@codemirror/commands'
import { SearchQuery, setSearchQuery, openSearchPanel } from '@codemirror/search'
const props = defineProps<{ modelValue: string; line?: number; dark?: boolean; notePath:string; workspaceId:string;noteTitle?:string;metadataVisible?:boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; file: [file: File, paste: boolean]; error:[message:string] }>()
const host = ref<HTMLElement>(); let view: EditorView | undefined; let external = false
let prefix='',lineOffset=0
function parts(text:string){return editorDocument(text,props.noteTitle,props.metadataVisible)}
function applyDocument(){if(!view)return;const next=parts(props.modelValue),absolute=view.state.selection.main.head+prefix.length;prefix=next.prefix;lineOffset=next.lineOffset;if(next.body!==view.state.doc.toString()){external=true;view.dispatch({changes:{from:0,to:view.state.doc.length,insert:next.body},selection:{anchor:Math.max(0,Math.min(next.body.length,absolute-prefix.length))},annotations:Transaction.addToHistory.of(false)});external=false}}
onMounted(() => {
 const initial=parts(props.modelValue);prefix=initial.prefix;lineOffset=initial.lineOffset
 view = new EditorView({ parent: host.value, state: EditorState.create({ doc: initial.body, extensions: [basicSetup, markdown(), admonitionCompletionExtension(), fenceCompletionExtension(), EditorView.lineWrapping,
  keymap.of([{ key: 'Mod-s', run: () => true }]),
  EditorView.theme({ '&': { height: '100%', fontSize: 'var(--note-font-size, 14px)', backgroundColor: 'transparent' }, '.cm-scroller': { overflow: 'auto', fontFamily: 'var(--note-font-family, "Cascadia Code", Consolas, monospace)', lineHeight: '1.9' }, '.cm-content': { padding: '26px 24px 120px', fontSynthesis: 'style', caretColor:'var(--note-caret-color,var(--text))' }, '.cm-cursor,.cm-dropCursor':{borderLeftColor:'var(--note-caret-color,var(--text))'}, '.cm-gutters': { backgroundColor: 'transparent', border: 'none', color: '#a5a8b2' }, '.cm-activeLine': { backgroundColor: '#7876e509' }, '.cm-activeLineGutter': { backgroundColor: 'transparent' }, '&.cm-focused': { outline: 'none' } }),
  EditorView.updateListener.of(update => { if (update.docChanged && !external) emit('update:modelValue', prefix+update.state.doc.toString()) }),
  EditorView.domEventHandlers({ paste: event => { const data=event.clipboardData;if(!data||!Array.from(data.items).some(item=>item.kind==='file')&&!data.files.length)return false;event.preventDefault();const workspace=props.workspaceId,path=props.notePath;void clipboardFileImports(data,()=>api.ClipboardHasFiles()).then(items=>{if(workspace===props.workspaceId&&path===props.notePath)for(const item of items)emit('file',item.file,item.screenshot)}).catch(error=>emit('error',String(error)));return true }, dragover: event => { if(event.dataTransfer?.types.includes(noteDragType)) {event.preventDefault();event.dataTransfer.dropEffect='copy';return true} return false }, drop: (event,editor) => {
   if(event.dataTransfer?.types.includes(noteDragType)) {
    event.preventDefault();event.stopPropagation()
    const note=readNoteDrag(event.dataTransfer)
    if(!note) {emit('error','无法识别拖入的笔记');return true}
    if(note.workspaceId!==props.workspaceId) {emit('error','请拖入当前工作区内的笔记以创建本地引用');return true}
    const position=editor.posAtCoords({x:event.clientX,y:event.clientY}) ?? editor.state.selection.main.head
    const text=noteReference(props.notePath,note)
    editor.dispatch({changes:{from:position,insert:text},selection:{anchor:position+text.length},userEvent:'input.drop'})
    editor.focus();return true
   }
   const files = event.dataTransfer?.files; if (files?.length) { event.preventDefault(); for (const file of Array.from(files)) emit('file', file, false); return true } return false } })
 ] }) })
})
watch(()=>[props.modelValue,props.metadataVisible,props.noteTitle],applyDocument)
watch(() => props.line, line => { if (view && line !== undefined) { const n = view.state.doc.line(Math.max(1,Math.min(line-lineOffset + 1, view.state.doc.lines))); view.dispatch({ selection: { anchor: n.from }, effects: EditorView.scrollIntoView(n.from, { y: 'center' }) }); view.focus() } })
function insert(text: string) { if (!view) return; const pos = view.state.selection.main; view.dispatch({ changes: { from: pos.from, to: pos.to, insert: text }, selection: { anchor: pos.from + text.length } }); view.focus() }
async function command(id:string) {
 if(!view) return
 if(id==='undo') undo(view)
 else if(id==='redo' || id==='redoAlternate') redo(view)
 else if(id==='selectAll') selectAll(view)
 else if(id==='find' || id==='replace') { openSearchPanel(view); if(id==='replace') (host.value?.querySelector('input[name="replace"]') as HTMLInputElement | null)?.focus(); return }
 else if(id==='copy' || id==='cut') {
  const range=view.state.selection.main, doc=view.state.doc, text=doc.sliceString(range.from,range.to)
  if(!text) return
  await navigator.clipboard.writeText(text)
  if(id==='cut' && view.state.doc===doc) { view.dispatch({changes:{from:range.from,to:range.to,insert:''},userEvent:'delete.cut'}) }
 } else if(id==='paste') insert(await navigator.clipboard.readText())
 else {
  const selection=view.state.selection.main, selected=view.state.sliceDoc(selection.from,selection.to)
  const marks:Record<string,string>={bold:'**',italic:'*',inlineCode:'`',strike:'~~'}
  const mark=marks[id]
  if(mark) { view.dispatch({changes:{from:selection.from,to:selection.to,insert:mark+selected+mark},selection:{anchor:selection.from+mark.length,head:selection.to+mark.length},userEvent:'input'}); }
  else if(['callout','heading','bulletList','orderedList','quote','horizontalRule','codeBlock','table','clearFormatting'].includes(id)) {
   const from=view.state.doc.lineAt(selection.from).from,to=view.state.doc.lineAt(selection.to>selection.from?selection.to-1:selection.to).to
   const block=view.state.sliceDoc(from,to),lines=block.split('\n')
   const insertions:Record<string,string>={callout:'\n> [!note] 提示\n'+(selected||'在这里填写内容').split('\n').map(line=>'> '+line).join('\n')+'\n',horizontalRule:'\n---\n',codeBlock:'```\n'+selected+'\n```',table:'| 列1 | 列2 |\n| --- | --- |\n| 内容 | 内容 |'}
   let text=insertions[id],start=selection.from,end=selection.to
   if(text===undefined){start=from;end=to;text=lines.map((line,i)=>id==='heading'?'# '+line.replace(/^#{1,6}\s+/, ''):id==='bulletList'?'- '+line.replace(/^(?:[-*+] |\d+\. )/,''):id==='orderedList'?(i+1)+'. '+line.replace(/^(?:[-*+] |\d+\. )/,''):id==='quote'?'> '+line.replace(/^>\s?/,''):line.replace(/^(?:#{1,6}\s+|>\s?|[-*+]\s+|\d+\.\s+)/,'').replace(/(\*\*|__|~~|`|\*)/g,'')).join('\n')}
   view.dispatch({changes:{from:start,to:end,insert:text},selection:{anchor:start,head:start+text.length},userEvent:'input'})
  }
  else if(id==='link') { const text='['+(selected || '链接文字')+'](https://)'; view.dispatch({changes:{from:selection.from,to:selection.to,insert:text},selection:{anchor:selection.from+text.length-9,head:selection.from+text.length-1},userEvent:'input'}) }
 }
 view.focus()
}
function focus() { view?.focus() }
function bookmarkPosition(){
 if(!view)return
 const position=view.state.selection.main.head,line=view.state.doc.lineAt(position)
 const full=(prefix+view.state.doc.toString()).split('\n'),sourceLine=line.number-1+lineOffset
 return {line:sourceLine,column:position-line.from,text:line.text,before:full[sourceLine-1]||'',after:full[sourceLine+1]||''}
}
function jumpToPosition(line:number,column=0){
 if(!view)return
 const target=view.state.doc.line(Math.max(1,Math.min(line-lineOffset+1,view.state.doc.lines))),pos=target.from+Math.max(0,Math.min(column,target.length))
 view.dispatch({selection:{anchor:pos},effects:EditorView.scrollIntoView(pos,{y:'center'})});view.focus()
}
function findMatches(term:string,index=0,regexp=false,caseSensitive=false){
 if(!view)return 0
 const query=new SearchQuery({search:term,caseSensitive,regexp,literal:true})
 view.dispatch({effects:setSearchQuery.of(query)})
 if(!term)return 0
 const cursor=query.getCursor(view.state.doc),matches:{from:number;to:number}[]=[]
 for(let result=cursor.next();!result.done;result=cursor.next())matches.push({from:result.value.from,to:result.value.to})
 if(matches.length){const match=matches[((index%matches.length)+matches.length)%matches.length];view.dispatch({selection:{anchor:match.from,head:match.to},effects:EditorView.scrollIntoView(match.from,{y:'center'})})}
 return matches.length
}
function selectedText(){if(!view)return;const range=view.state.selection.main;return {from:range.from+prefix.length,to:range.to+prefix.length,text:view.state.sliceDoc(range.from,range.to)}}
function replaceRange(from:number,to:number,expected:string,text:string){if(!view||from<prefix.length||to>prefix.length+view.state.doc.length||view.state.sliceDoc(from-prefix.length,to-prefix.length)!==expected)return false;view.dispatch({changes:{from:from-prefix.length,to:to-prefix.length,insert:text},selection:{anchor:from-prefix.length,head:from-prefix.length+text.length},userEvent:'input'});return true}
defineExpose({ insert, command, focus, bookmarkPosition, jumpToPosition, findMatches,selectedText,replaceRange })
onBeforeUnmount(() => view?.destroy())
</script>
<template><div ref="host" class="codemirror-host" /></template>
