import {completionStatus} from '@codemirror/autocomplete'
import {ensureSyntaxTree,syntaxTree} from '@codemirror/language'
import {Prec} from '@codemirror/state'
import {EditorView,keymap} from '@codemirror/view'

export function fenceCompletionExtension(){return [
 Prec.highest(EditorView.inputHandler.of((view,from,to,text)=>{
  if(text!=='`'||from!==to||view.state.selection.ranges.length!==1)return false
  const line=view.state.doc.lineAt(from),before=line.text.slice(0,from-line.from)
  if(!/^ {0,3}(?:>[ \t]*)*``$/.test(before)||line.text.slice(from-line.from).trim())return false
  const tree=ensureSyntaxTree(view.state,from,20)||syntaxTree(view.state)
  for(let node=tree.resolveInner(from,-1);node;node=node.parent!){if(['FencedCode','CodeBlock','InlineCode'].includes(node.name))return false}
  const prefix=before.slice(0,-2),close=prefix+'```'
  // Reuse an existing closing fence immediately below this opener.
  for(let number=line.number+1;number<=view.state.doc.lines;number++){
   const next=view.state.doc.line(number).text
   if(next===close)return false
   if(next.trim())break
  }
  const newline=view.state.lineBreak
  view.dispatch({changes:{from,to:line.to,insert:'`'+newline+prefix+newline+close},selection:{anchor:from+1},userEvent:'input.type'})
  return true
 })),
 Prec.high(keymap.of([{key:'Enter',run(view){
  const range=view.state.selection.main
  if(!range.empty||view.state.selection.ranges.length!==1||completionStatus(view.state)==='active')return false
  const line=view.state.doc.lineAt(range.head),match=/^( {0,3}(?:>[ \t]*)*)`{3}[^`]*$/.exec(line.text)
  if(!match||range.head!==line.to||line.number+2>view.state.doc.lines)return false
  const tree=ensureSyntaxTree(view.state,range.head,20)||syntaxTree(view.state)
  let opener=false
  for(let node=tree.resolveInner(range.head,-1);node;node=node.parent!){if(node.name==='FencedCode'){opener=node.from===line.from+match[1]!.length;break}}
  if(!opener)return false
  const body=view.state.doc.line(line.number+1),close=view.state.doc.line(line.number+2)
  if(body.text!==match[1]||close.text!==match[1]+'```')return false
  view.dispatch({selection:{anchor:body.to},scrollIntoView:true});return true
 }}])),
]}
