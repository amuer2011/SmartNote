<script setup lang="ts">
import {onMounted,onBeforeUnmount,ref,watch,computed} from 'vue'
import cytoscape from 'cytoscape'
import GraphAnnotationEditor from './GraphAnnotationEditor.vue'
import {api, type Note, type Edge, type GraphLayout,type GraphAnnotations} from '../services/api'
const props=defineProps<{workspaceId?:string;notes:Note[];edges:Edge[];selected?:string;global:boolean;filter:string;annotations?:Record<string,string>;linking?:boolean;revision?:string;active?:boolean}>()
const emit=defineEmits<{open:[id:string];inspect:[edge:Edge];inspectNode:[id:string];connect:[pair:{source:string;target:string}];changed:[metadata:GraphAnnotations]}>()
const root=ref<HTMLElement>(),host=ref<HTMLElement>();let graph:cytoscape.Core|undefined,observer:ResizeObserver|undefined,frame=0,needsLayout=true
const positions=new Map<string,{x:number;y:number}>()
let viewport:{zoom:number;pan:{x:number;y:number}}|undefined
let scope='',loaded=false,disposed=false
const saveError=ref(''),linkSource=ref('')
const chosen=ref('')
const selectedNote=computed(()=>props.notes.find(note=>note.id===chosen.value))
const editing=ref<{id:string;kind:'node'|'edge';text:string;revision:string;source?:string;target?:string}>()
const annotationSaving=ref(false),annotationError=ref(''),editorPosition=ref({left:8,top:8,width:310,maxHeight:260})
let themeObserver:MutationObserver|undefined
function selectElement(id:string) {
 chosen.value=id
 graph?.elements().unselect();graph?.getElementById(id).select();graph?.nodes('[kind="annotation"]').removeClass('annotation-selected');graph?.getElementById(annotationId(id)).addClass('annotation-selected')
 root.value?.focus({preventScroll:true})
}
function noteNodes(){return graph!.nodes('[kind="note"]')}
function resolveNodeHit(target:cytoscape.NodeSingular,point?:{x:number;y:number}){
 if(!graph||!point||target.data('kind')!=='annotation')return target
 let hit=target,distance=Infinity
 // Annotation text may cross another note's circle in a dense or saved layout.
 // Keep the circle actionable even when the annotation is painted above it.
 noteNodes().forEach(node=>{
  const position=node.position(),rx=node.outerWidth()/2+2/graph!.zoom(),ry=node.outerHeight()/2+2/graph!.zoom()
  const score=((point.x-position.x)/rx)**2+((point.y-position.y)/ry)**2
  if(score<=1&&score<distance){hit=node;distance=score}
 })
 return hit
}
function annotationId(owner:string){return 'annotation-'+JSON.stringify(owner)}
function annotationSummary(text:string){
 const measure=document.createElement('canvas').getContext('2d')!
 measure.font='12px Microsoft YaHei, Segoe UI, sans-serif'
 const lines=[''],characters=[...text.trim()];let truncated=false
 for(const character of characters){
  const line=lines.length-1
  if(character==='\n'||measure.measureText(lines[line]+character).width>190){
   if(lines.length===4){truncated=true;break}
   lines.push(character==='\n'?'':character)
  }else lines[line]+=character
 }
 if(truncated){while(measure.measureText(lines[3]+'…').width>190)lines[3]=lines[3].slice(0,-1);lines[3]+='…'}
 return lines.join('\n')
}
// Annotation glyphs are native Cytoscape elements. They share the graph's renderer,
// viewport transform and paint frame, rather than a separately scheduled DOM layer.
function syncAnnotations(node?:cytoscape.NodeSingular){
 if(!graph)return
 const owners=node?node.union(node.connectedEdges()):graph.elements('[kind="note"],edge')
 graph.batch(()=>owners.forEach(owner=>{
  const label=graph!.getElementById(annotationId(owner.id()));if(!label.length)return
  if(owner.isNode()){
   const bounds=owner.boundingBox({includeLabels:true}),height=label.boundingBox({includeLabels:true}).h
   label.position({x:owner.position('x'),y:bounds.y2+10+height/2})
  }else label.position((owner as cytoscape.EdgeSingular).midpoint())
 }))
 if(editing.value)updateEditorPosition()
}
function scheduleLabels(){if(editing.value)updateEditorPosition()}
function updateEditorPosition(){
 if(!graph||!host.value||!editing.value)return
 const width=host.value.clientWidth,height=host.value.clientHeight
 const element=graph.getElementById(annotationId(editing.value.id)).length?graph.getElementById(annotationId(editing.value.id)):graph.getElementById(editing.value.id)
 if(!element.length){editing.value=undefined;return}
 const p=element.isNode()?element.renderedPosition():(()=>{const m=(element as cytoscape.EdgeSingular).midpoint();return {x:m.x*graph!.zoom()+graph!.pan().x,y:m.y*graph!.zoom()+graph!.pan().y}})()
 const w=Math.min(330,width-16),h=Math.min(260,height-16)
 editorPosition.value={left:Math.max(8,Math.min(width-w-8,p.x+18)),top:Math.max(8,Math.min(height-h-8,p.y+18)),width:w,maxHeight:h}
}
function startEditing(){
 if(editing.value||props.linking||!graph||!chosen.value)return
 const element=graph.getElementById(chosen.value);if(!element.length)return
 const kind=element.isNode()?'node':'edge',relation=element.data('relation') as Edge|undefined
 editing.value={id:element.id(),kind,text:kind==='node'?props.annotations?.[element.id()]||'':relation?.annotation||'',revision:props.revision||'',source:relation?.source,target:relation?.target}
 annotationError.value='';updateEditorPosition()
}
function keyboard(event:KeyboardEvent){
 if(event.isComposing||event.defaultPrevented)return
 if(event.target instanceof Element&&event.target.closest('button,input,textarea,select,[contenteditable="true"]'))return
 if(event.key==='Enter'&&selectedNote.value&&!props.linking&&!editing.value){event.preventDefault();event.stopPropagation();emit('open',selectedNote.value.id);return}
 if(event.code==='Space'||event.key===' '){event.preventDefault();event.stopPropagation();startEditing()}
}
async function saveAnnotation(text:string){
 const edit=editing.value,workspace=props.workspaceId
 if(!edit||!workspace||annotationSaving.value)return
 annotationSaving.value=true;annotationError.value=''
 try{
  const metadata=await api.SaveGraphAnnotation({workspaceId:workspace,revision:edit.revision,...(edit.kind==='node'?{noteId:edit.id}:{source:edit.source,target:edit.target}),text})
  if(disposed||workspace!==props.workspaceId)return
  editing.value=undefined;emit('changed',metadata);root.value?.focus({preventScroll:true})
 }catch(error){if(!disposed&&workspace===props.workspaceId)annotationError.value='保存失败，输入内容已保留：'+String(error)}finally{annotationSaving.value=false}
}
function cancelEditing(){if(annotationSaving.value)return;editing.value=undefined;root.value?.focus({preventScroll:true})}
function activateNode(target:cytoscape.NodeSingular){
 if(props.linking||editing.value)return
 if(target.data('kind')==='annotation'){selectElement(target.data('owner'));startEditing()}
 else emit('open',target.id())
}
function updateTheme(){
 if(!graph||!root.value)return
 const style=getComputedStyle(root.value)
 const background=style.getPropertyValue('--bg').trim()||'#fff',dark=document.documentElement.dataset.theme==='dark'
 graph.style().selector('node[kind="note"]').style({'color':style.getPropertyValue('--text').trim()||'#303342','text-outline-color':background})
 .selector('node[kind="annotation"]').style({'color':dark?'#e4b76c':'#9b5c16','text-background-color':background,'text-outline-color':background}).update()
 scheduleLabels()
}
let layouts:Record<string,GraphLayout>={},saveTimer:ReturnType<typeof setTimeout>|undefined,saveQueue=Promise.resolve()
function persist(){
 clearTimeout(saveTimer);saveTimer=undefined
 if(!graph||!scope||!props.workspaceId||!noteNodes().length)return
 remember()
 const key=scope,workspace=props.workspaceId
 const value:GraphLayout={positions:Object.fromEntries(positions),zoom:graph.zoom(),pan:{...graph.pan()}}
 layouts[key]=value
 saveQueue=saveQueue.then(()=>api.SaveGraphLayout(workspace,key,value)).then(()=>{saveError.value=''}).catch(e=>{saveError.value='拓扑布局保存失败：'+String(e)})
}
function scheduleSave(){if(disposed)return;clearTimeout(saveTimer);saveTimer=setTimeout(persist,350)}
const signature=computed(()=>JSON.stringify([props.notes.map(n=>[n.id,n.title]),props.edges,props.selected,props.global,props.filter,props.annotations,props.linking]))
function remember(){if(!graph)return;noteNodes().forEach(node=>{positions.set(node.id(),{...node.position()})});viewport={zoom:graph.zoom(),pan:{...graph.pan()}}}
function layoutWhenVisible(){
 cancelAnimationFrame(frame)
 frame=requestAnimationFrame(()=>{
  if(!graph||!host.value)return
  const rect=host.value.getBoundingClientRect();if(rect.width<100||rect.height<100)return
  graph.resize()
  scheduleLabels();if(!needsLayout)return
  const count=noteNodes().length;if(!count)return
  const nodes=noteNodes(),linked=nodes.filter(node=>node.degree()>0),isolated=nodes.filter(node=>node.degree()===0)
  const angle=Math.PI*(3-Math.sqrt(5))
  // Spiral seeds avoid rows and columns, including workspaces without any links.
  nodes.forEach((node,i)=>{const radius=58*Math.sqrt(i);node.position({x:Math.cos(i*angle)*radius,y:Math.sin(i*angle)*radius})})
  if(linked.length)graph.elements().filter(element=>element.isEdge()||element.degree()>0).layout({name:'cose',fit:false,animate:false,randomize:false,nodeDimensionsIncludeLabels:true,nodeRepulsion:()=>2800,idealEdgeLength:()=>65,edgeElasticity:()=>70,componentSpacing:35,gravity:.7,numIter:650,padding:30}).run()
  const bounds=linked.boundingBox({includeLabels:true}),center=linked.length?{x:(bounds.x1+bounds.x2)/2,y:(bounds.y1+bounds.y2)/2}:{x:0,y:0}
  const base=linked.length?Math.max(bounds.w,bounds.h)/4+30:0
  isolated.forEach((node,i)=>{const radius=Math.sqrt(base*base+Math.pow(58,2)*(i+(linked.length?1:0)));node.position({x:center.x+Math.cos(i*angle)*radius,y:center.y+Math.sin(i*angle)*radius})})
  // Resolve label collisions with a small gap, without imposing a grid.
  for(let pass=0;pass<30;pass++){
   let moved=false
   for(let i=0;i<nodes.length;i++)for(let j=i+1;j<nodes.length;j++){
    const first=nodes[i],second=nodes[j],a=first.boundingBox({includeLabels:true}),b=second.boundingBox({includeLabels:true})
    const dx=(b.x1+b.x2-a.x1-a.x2)/2,dy=(b.y1+b.y2-a.y1-a.y2)/2
    const ox=(a.w+b.w)/2+12-Math.abs(dx),oy=(a.h+b.h)/2+12-Math.abs(dy)
    if(ox<=0||oy<=0)continue
    const shift=ox<oy?{x:(dx>=0?1:-1)*(ox/2+.5),y:0}:{x:0,y:(dy>=0?1:-1)*(oy/2+.5)}
    first.position({x:first.position('x')-shift.x,y:first.position('y')-shift.y});second.position({x:second.position('x')+shift.x,y:second.position('y')+shift.y});moved=true
   }
   if(!moved)break
  }
  syncAnnotations();graph.fit(noteNodes().union(graph.nodes('[kind="annotation"]')),60);if(graph.zoom()>1.25)graph.zoom({level:1.25,renderedPosition:{x:rect.width/2,y:rect.height/2}})
  needsLayout=false;remember();scheduleSave()
 })
}
function relayout(){needsLayout=true;layoutWhenVisible()}
function render(){
 if(!host.value||!loaded||disposed)return
 host.value.title=''
 if(saveTimer)persist()
 remember();graph?.destroy()
 let edges=props.edges.filter(e=>['MARKDOWN_LINK','USER_CONFIRMED'].includes(e.type)&&(!props.filter||e.type===props.filter)),notes=props.notes
 if(!props.global&&props.selected){edges=edges.filter(e=>e.source===props.selected||e.target===props.selected);const ids=new Set([props.selected,...edges.flatMap(e=>[e.source,e.target])]);notes=notes.filter(n=>ids.has(n.id))}
 const ids=new Set(notes.map(n=>n.id));edges=edges.filter(e=>ids.has(e.source)&&ids.has(e.target))
 const nextScope=JSON.stringify([props.global,props.filter,props.global?'':props.selected]);if(nextScope!==scope){
  scope=nextScope;positions.clear();chosen.value='';editing.value=undefined
  const saved=layouts[scope]
  if(saved)for(const [id,point] of Object.entries(saved.positions||{}))positions.set(id,{...point})
  viewport=saved?{zoom:saved.zoom,pan:{...saved.pan}}:undefined
  needsLayout=!saved
 }
 // Drop removed notes without disturbing saved positions for the current scope.
 for(const id of positions.keys())if(!props.notes.some(n=>n.id===id))positions.delete(id)
 let nextX=positions.size?Math.max(...[...positions.values()].map(p=>p.x))+120:0,newIndex=0
 graph=cytoscape({container:host.value,elements:[...notes.map(n=>({data:{id:n.id,kind:'note',label:n.title,selected:n.id===props.selected?'yes':'no'},position:positions.get(n.id)||{x:nextX+Math.cos(newIndex*2.39996)*70*Math.sqrt(newIndex),y:Math.sin(newIndex*2.39996)*70*Math.sqrt(newIndex++)}})),...edges.map(e=>({data:{id:'edge-'+JSON.stringify([e.source,e.target,e.type]),source:e.source,target:e.target,label:e.annotation?.slice(0,80)||'',type:e.type,relation:e}}))],style:[{selector:'node[kind="note"]',style:{'background-color':'#9692d5','text-events':'yes',label:'data(label)',color:'#303342','font-size':14,'font-weight':700,'font-family':'Microsoft YaHei, Segoe UI, sans-serif','text-outline-width':2,'text-outline-color':'#fff','text-valign':'bottom','text-margin-y':12,'text-wrap':'wrap','text-max-width':'180px',width:18,height:18}},{selector:'node[kind="note"][selected="yes"]',style:{'background-color':'#7770ca',width:30,height:30,'border-width':8,'border-color':'#eeecfb'}},{selector:'edge',style:{width:1.3,'line-color':'#d9d8e9','curve-style':'bezier',label:'','font-size':10,color:'#8b86ae','text-rotation':'autorotate','text-background-color':'#fff','text-background-opacity':0,'text-margin-y':-8,'text-background-padding':'3px'}},{selector:'edge[type="SEMANTIC_SIMILAR"]',style:{'line-style':'dashed','line-color':'#aaa1d4',width:1.5}},{selector:'edge[type="USER_CONFIRMED"]',style:{'line-color':'#6caa92',width:2.3}},{selector:'node[kind="note"]:selected',style:{'border-width':3,'border-color':'#8174c8'}},{selector:'edge:selected',style:{'line-color':'#8174c8',width:3}},{selector:'node[kind="annotation"]',style:{width:1,height:1,'background-opacity':0,'border-width':0,label:'data(label)','font-size':12,'font-weight':400,'font-family':'Microsoft YaHei, Segoe UI, sans-serif',color:'#9b5c16','text-wrap':'wrap','text-max-width':'200px','text-overflow-wrap':'anywhere','text-valign':'center','text-halign':'center','text-background-color':'#fff','text-background-opacity':1,'text-background-padding':'4px','text-background-shape':'roundrectangle','text-outline-width':1,'text-outline-color':'#fff','text-events':'yes','z-index-compare':'manual','z-index':100}},{selector:'node[kind="annotation"].annotation-selected',style:{'text-border-width':1,'text-border-color':'#8174c8','text-border-opacity':1}}],layout:{name:'preset',fit:false},minZoom:.05,maxZoom:3,autoungrabify:!!props.linking})
 if(viewport){graph.zoom(viewport.zoom);graph.pan(viewport.pan)}
 graph.batch(()=>{
  const additions:cytoscape.ElementDefinition[]=[]
  graph!.elements('[kind="note"],edge').forEach(owner=>{
   const text=owner.group()==='nodes'?props.annotations?.[owner.id()]:(owner.data('relation') as Edge).annotation
   if(text?.trim())additions.push({group:'nodes',data:{id:annotationId(owner.id()),kind:'annotation',owner:owner.id(),label:annotationSummary(text),fullText:text},position:{x:0,y:0},grabbable:false,selectable:false})
  })
  graph!.add(additions)
 })
 graph.on('tap','node',e=>{
  const target=resolveNodeHit(e.target,e.position),owner=target.data('kind')==='annotation'?graph!.getElementById(target.data('owner')):target
  if(owner.isEdge()){
   selectElement(owner.id());emit('inspect',owner.data('relation'))
   if(e.originalEvent instanceof MouseEvent&&e.originalEvent.detail===2)activateNode(target)
   return
  }
  const id=owner.id()
  if(props.linking){if(!linkSource.value){linkSource.value=id;return};if(linkSource.value!==id){emit('connect',{source:linkSource.value,target:id});linkSource.value=''}}
  else {selectElement(id);emit('inspectNode',id)}
  // MouseEvent.detail identifies an actual double click, even if the graph's
  // internal multi-click timer survives closing and reopening this view.
  if(e.originalEvent instanceof MouseEvent&&e.originalEvent.detail===2)activateNode(target)
 })
 graph.on('dbltap','node',e=>{
  if(e.originalEvent instanceof MouseEvent)return
  activateNode(resolveNodeHit(e.target,e.position))
 })
 graph.on('tap','edge',e=>{selectElement(e.target.id());emit('inspect',e.target.data('relation'))})
 graph.on('mouseover','node[kind="annotation"]',e=>{if(host.value)host.value.title=e.target.data('fullText')})
 graph.on('mouseout','node[kind="annotation"]',()=>{if(host.value)host.value.title=''})
 graph.on('position','node[kind="note"]',e=>syncAnnotations(e.target))
 graph.on('position pan zoom',scheduleSave)
 graph.on('pan zoom resize',scheduleLabels)
 if(chosen.value)selectElement(chosen.value)
 updateTheme();syncAnnotations();layoutWhenVisible()

}
onMounted(async()=>{
 try{if(props.workspaceId)layouts=(await api.GetGraphLayouts(props.workspaceId))||{}}catch(e){saveError.value='拓扑布局读取失败：'+String(e)}
 if(disposed)return
 loaded=true;render();themeObserver=new MutationObserver(updateTheme);themeObserver.observe(document.documentElement,{attributes:true,attributeFilter:['data-theme','style','class']});observer=new ResizeObserver(layoutWhenVisible);if(host.value)observer.observe(host.value)
})
watch(signature,render)
watch(()=>props.linking,()=>{linkSource.value='';editing.value=undefined})
watch(()=>props.active,active=>{if(!active)editing.value=undefined})
onBeforeUnmount(()=>{persist();disposed=true;clearTimeout(saveTimer);cancelAnimationFrame(frame);observer?.disconnect();themeObserver?.disconnect();graph?.destroy()})
function getViewport(){return graph?{zoom:graph.zoom(),pan:{...graph.pan()},positions:noteNodes().map(node=>({id:node.id(),...node.position()}))}:undefined}
function setViewport(zoom:number,pan:{x:number;y:number}){graph?.zoom(zoom);graph?.pan(pan)}
defineExpose({getViewport,setViewport,relayout})
</script>
<template><div ref="root" class="knowledge-graph" tabindex="0" aria-label="知识拓扑画布，选中节点或连线后按空格编辑备注" @keydown="keyboard"><span v-if="linking" class="graph-link-hint" role="status">{{linkSource?'再点击一篇笔记完成连线':'依次点击两篇笔记建立关联'}}</span><span v-if="saveError" class="graph-save-error" role="status">{{saveError}}</span><button class="secondary-button graph-relayout" aria-label="重新布局拓扑" @click="relayout">重新布局</button><div ref="host" class="graph-canvas" />
<div v-if="selectedNote&&!linking&&!editing" class="graph-node-actions" @pointerdown.stop><span :title="selectedNote.title">{{selectedNote.title}}</span><button class="secondary-button" aria-label="打开选中笔记" @click="emit('open',selectedNote.id)">打开笔记 <kbd>Enter</kbd></button></div>
<GraphAnnotationEditor v-if="editing" :key="editing.id" v-bind="editorPosition" :kind="editing.kind" :text="editing.text" :saving="annotationSaving" :error="annotationError" @save="saveAnnotation" @cancel="cancelEditing" /></div></template>
<style scoped>.knowledge-graph{flex:1;min-height:0;position:relative;display:flex;flex-direction:column}.graph-canvas{flex:1;min-height:0}.graph-save-error{position:absolute;left:16px;top:10px;z-index:2;color:var(--danger,#b44);font-size:12px;max-width:70%}.graph-link-hint{position:absolute;left:16px;top:12px;z-index:2;padding:8px 12px;border-radius:7px;background:var(--accent-bg);color:var(--accent);font-size:.85rem}.graph-relayout{position:absolute;right:16px;top:10px;z-index:2}.knowledge-graph{outline:none}.graph-node-actions{position:absolute;left:16px;bottom:12px;z-index:3;display:flex;align-items:center;gap:12px;max-width:calc(100% - 32px);padding:7px 10px;border:1px solid var(--line);border-radius:8px;background:var(--bg);box-shadow:var(--shadow);font-size:12px}.graph-node-actions>span{max-width:240px;min-width:0;overflow:hidden;white-space:nowrap;text-overflow:ellipsis;color:var(--secondary)}.graph-node-actions button{display:flex;gap:8px;align-items:center;white-space:nowrap;padding:5px 9px;flex-shrink:0}</style>
