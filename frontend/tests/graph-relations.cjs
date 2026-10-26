const {chromium}=require('playwright');
(async()=>{
 const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
 try{
  const page=await browser.newPage({viewport:{width:1500,height:1000}}),errors=[];
  page.on('pageerror',e=>errors.push(String(e)));
  await page.addInitScript(()=>{
   const notes=[['a','数据库排查'],['b','SQL性能调优'],['c','旅行计划']].map(([id,title])=>({id,title,path:title+'.md',content:'# '+title+'\n正文保持原样。',hash:id+'-hash',tags:[],project:'',updated:new Date().toISOString()}));
   const workspace={id:'w',name:'图谱测试',path:'D:/IsolatedGraph'},pairs=()=>JSON.parse(localStorage.getItem('graph-confirmed')||'[]');
   window.graphCalls=[];window.graphWrites=[];window.originalNotes=notes;window.externalEvents={};
   const app={
    Snapshot:async()=>({settings:{workspaces:[workspace],activeId:'w',theme:'light',picGoUrl:'',ui:{fontSize:14,collapsedWorkspaces:[],entryOrder:{}}},notes:structuredClone(notes),directories:[],tasks:[],attachments:[]}),
    DataDirectory:async()=> 'D:/IsolatedGraphData',ListWorkspaceFiles:async()=>({notes,directories:[]}),GetBookmarks:async()=>[],DueReminders:async()=>[],AutoArchive:async()=>null,SetUIPreferences:async()=>{},
    GetHolidayYear:async year=>({year,holiday:{},type:{}}),GetLunarYear:async year=>({year,version:1,days:{}}),GetTaskArchive:async()=>[],
    Graph:async()=>pairs().map(r=>({...r,type:'USER_CONFIRMED',annotation:JSON.parse(localStorage.getItem('graph-annotations')||'{"nodes":{},"relations":{}}').relations[[r.source,r.target].sort().join('|')]})),
    GetGraphAnnotations:async()=>JSON.parse(localStorage.getItem('graph-annotations')||'{"revision":"","nodes":{},"relations":{}}'),
    SaveGraphAnnotation:async input=>{if(window.failAnnotation){window.failAnnotation=false;throw Error('测试保存失败')}const value=await app.GetGraphAnnotations();if(value.revision!==input.revision)throw Error('CONFLICT');const key=input.noteId||[input.source,input.target].sort().join('|');(input.noteId?value.nodes:value.relations)[key]=input.text;value.revision=String(Date.now());localStorage.setItem('graph-annotations',JSON.stringify(value));return value},
    GetGraphLayouts:async()=>JSON.parse(localStorage.getItem('graph-layouts')||'{}'),
    SaveGraphLayout:async(w,key,value)=>{const layouts=await app.GetGraphLayouts();layouts[key]=value;localStorage.setItem('graph-layouts',JSON.stringify(layouts))},
    SuggestGraphRelations:async(w,id,threshold)=>{
     window.graphCalls.push({w,id,threshold});const wait=window.delayNext||0;window.delayNext=0;
     if(wait)await new Promise(resolve=>setTimeout(resolve,wait));
     return {message:'全文向量测试建议',edges:threshold>.84||pairs().length?[]:[{source:'a',target:'b',type:'SEMANTIC_SIMILAR',score:.84,sourceHash:'a-hash',targetHash:'b-hash'}]};
    },
    SetGraphRelation:async input=>{
     window.graphWrites.push(input);const a=notes.find(n=>n.id===input.source),b=notes.find(n=>n.id===input.target);
     if(input.workspaceId!=='w'||a?.hash!==input.sourceHash||b?.hash!==input.targetHash)throw Error('CONFLICT');
     const key=r=>[r.source,r.target].sort().join('|'),next=pairs().filter(r=>key(r)!==key(input));
     if(input.confirmed)next.push({source:input.source,target:input.target});localStorage.setItem('graph-confirmed',JSON.stringify(next));
    }
   };
   window.go={main:{App:app}};window.runtime={EventsOn:(name,fn)=>{window.externalEvents[name]=fn;return()=>{}},BrowserOpenURL:()=>{}};
  });
  await page.goto(process.env.SMARTNOTE_TEST_URL||'http://127.0.0.1:4175');
  const panel=page.locator('.graph-relations'),canvas=page.locator('.graph-main-overlay .graph-canvas');
  async function open(){await page.locator('.tree-file').filter({hasText:'数据库排查'}).first().click();await page.getByRole('tab',{name:'知识拓扑',exact:true}).click();await panel.waitFor();}
  await page.getByRole('tab',{name:'知识拓扑',exact:true}).click();await canvas.waitFor();
  if(await panel.count())throw Error('Recommendation panel visible by default');
  await page.waitForFunction(()=>document.querySelector('.graph-canvas')._cyreg.cy.nodes().length===3);
  if((await page.evaluate(()=>window.graphCalls.length))!==0)throw Error('Topology requested model suggestions');
  if(await page.getByText('语义候选',{exact:true}).count()||await page.getByLabel('拓扑最低余弦相似度').count())throw Error('Recommendation UI retained');
  const before=await canvas.evaluate(el=>{const cy=el._cyreg.cy;cy.getElementById('a').position({x:123,y:-27});cy.zoom(1.12);cy.pan({x:87,y:92});return {point:{...cy.getElementById('a').position()},zoom:cy.zoom(),pan:{...cy.pan()}}});
  await page.getByRole('button',{name:'手动连线',exact:true}).click();
  await canvas.evaluate(el=>{const cy=el._cyreg.cy;cy.getElementById('a').emit('tap');cy.getElementById('b').emit('tap')});
  await panel.waitFor();await panel.getByRole('button',{name:'保存关联',exact:true}).click();
  await page.waitForFunction(()=>document.querySelector('.graph-canvas')._cyreg.cy.edges('[type="USER_CONFIRMED"]').length===1);
  await panel.getByRole('button',{name:'关闭关联与备注',exact:true}).click();
  async function edit(id,kind,text){
   await canvas.evaluate((el,id)=>{el._cyreg.cy.getElementById(id).emit('tap')},id);
   await page.locator('.knowledge-graph').press('Space');
   const editor=page.getByRole('dialog',{name:'编辑拓扑备注'});
   await editor.getByRole('textbox',{name:kind==='node'?'图上笔记备注':'图上连线备注'}).fill(text);
   return editor;
  }
  let edgeId=await canvas.evaluate(el=>el._cyreg.cy.edges()[0].id());
  let editor=await edit(edgeId,'edge','SQL排查的补充资料');
  await editor.getByRole('textbox').press('Enter');await editor.waitFor({state:'hidden'});
  await page.waitForFunction(()=>document.querySelector('.graph-canvas')._cyreg.cy.nodes('[kind="annotation"]').some(n=>n.data('fullText')==='SQL排查的补充资料'));
  const edgeLabelPoint=await canvas.evaluate(el=>{const cy=el._cyreg.cy,edge=cy.edges()[0];return {...cy.nodes('[kind="annotation"]').filter(n=>n.data('owner')===edge.id())[0].renderedPosition()}});
  await canvas.dblclick({position:edgeLabelPoint});editor=page.getByRole('dialog',{name:'编辑拓扑备注'});await editor.waitFor();
  if(await editor.getByRole('textbox').getAttribute('aria-label')!=='图上连线备注'||await editor.getByRole('textbox').inputValue()!=='SQL排查的补充资料')throw Error('Edge annotation double click did not edit');
  await editor.getByRole('textbox').press('Escape');
  const after=await canvas.evaluate(el=>{const cy=el._cyreg.cy;return {point:{...cy.getElementById('a').position()},zoom:cy.zoom(),pan:{...cy.pan()}}});
  if(JSON.stringify(before)!==JSON.stringify(after))throw Error('Editing relationship moved graph: '+JSON.stringify({before,after}));
  editor=await edit('a','node','数据库排查知识入口');
  await page.evaluate(()=>window.failAnnotation=true);await editor.getByRole('textbox').press('Enter');
  await editor.getByRole('alert').waitFor();
  if(await editor.getByRole('textbox').inputValue()!=='数据库排查知识入口')throw Error('Failed save lost draft');
  await editor.getByRole('textbox').press('Enter');await editor.waitFor({state:'hidden'});
  await page.waitForFunction(()=>document.querySelector('.graph-canvas')._cyreg.cy.nodes('[kind="annotation"]').some(n=>n.data('fullText')==='数据库排查知识入口'));
  if(await panel.count())throw Error('Selecting notes opened sidebar');
  editor=await edit('b','node','取消的备注');await editor.getByRole('textbox').press('Escape');await editor.waitFor({state:'hidden'});
  if(!await canvas.isVisible())throw Error('Escape closed topology');
  if((await page.evaluate(()=>window.originalNotes[0].content))!=='# 数据库排查\n正文保持原样。')throw Error('Graph annotation changed Markdown');
  const aPoint=await canvas.evaluate(el=>({...el._cyreg.cy.getElementById('a').renderedPosition()}));
  await canvas.click({position:aPoint});await page.keyboard.press('Space');
  editor=page.getByRole('dialog',{name:'编辑拓扑备注'});await editor.waitFor();
  await editor.getByRole('textbox').fill('多行备注');await editor.getByRole('textbox').press('Shift+Enter');await editor.getByRole('textbox').press('A');
  await editor.getByRole('textbox').dispatchEvent('keydown',{key:'Enter',code:'Enter',isComposing:true});
  if(!await editor.isVisible()||!(await editor.getByRole('textbox').inputValue()).includes('\n'))throw Error('Multiline/IME protection failed');
  await editor.getByRole('textbox').press('Escape');
  const visual=await canvas.evaluate(el=>{const n=el._cyreg.cy.getElementById('a');return {label:n.data('label'),weight:n.style('font-weight'),size:n.style('font-size')}});
  if(visual.label!=='数据库排查'||parseFloat(visual.size)<14||parseInt(visual.weight)<600)throw Error('Title emphasis missing '+JSON.stringify(visual));
  await page.screenshot({path:'docs/preview-inline-graph-v0.12.5.png'});
  const lightColor=await canvas.evaluate(el=>el._cyreg.cy.getElementById('a').style('color'));
  const lightAnnotation=await canvas.evaluate(el=>el._cyreg.cy.nodes('[kind="annotation"]')[0].style('color'));
  await page.evaluate(()=>document.documentElement.dataset.theme='dark');
  await page.waitForFunction(old=>document.querySelector('.graph-canvas')._cyreg.cy.nodes('[kind="annotation"]')[0].style('color')!==old,lightAnnotation);
  await page.waitForFunction(old=>document.querySelector('.graph-canvas')._cyreg.cy.getElementById('a').style('color')!==old,lightColor);
  await page.screenshot({path:'docs/preview-inline-graph-dark-v0.12.5.png'});
  await page.evaluate(()=>document.documentElement.dataset.theme='light');
  if(await page.locator('.graph-annotation,.annotation-leaders').count())throw Error('Separate DOM annotation layer retained');
  const nativeCheck=await canvas.evaluate(el=>{
   const cy=el._cyreg.cy,helpers=cy.nodes('[kind="annotation"]');
   const owner=cy.getElementById('a'),nodeLabel=helpers.filter(n=>n.data('owner')==='a')[0];
   const edge=cy.edges()[0],edgeLabel=helpers.filter(n=>n.data('owner')===edge.id())[0];
   if(helpers.some(n=>n.data('label').includes('笔记备注')||n.data('label').includes('连线备注')))throw Error('Type headings retained');
   if(edgeLabel.style('text-background-opacity')!=='1'||edgeLabel.style('z-index-compare')!=='manual'||parseFloat(edgeLabel.style('z-index'))<100)throw Error('Line masking missing');
   const original={...owner.position()},offset={x:nodeLabel.position('x')-owner.position('x'),y:nodeLabel.position('y')-owner.position('y')};
   for(let i=0;i<60;i++){
    owner.position({x:original.x+i*.9,y:original.y+i*.4});
    if(Math.abs(nodeLabel.position('x')-owner.position('x')-offset.x)>.1||Math.abs(nodeLabel.position('y')-owner.position('y')-offset.y)>.1)throw Error('Node annotation did not follow synchronously');
    const middle=edge.midpoint();if(Math.hypot(middle.x-edgeLabel.position('x'),middle.y-edgeLabel.position('y'))>.1)throw Error('Edge annotation left its line');
   }
   owner.position(original);
   cy.zoom(1.3);cy.panBy({x:25,y:22});
   const middle=edge.midpoint(),rendered=edgeLabel.renderedPosition(),pan=cy.pan();
   if(Math.hypot(rendered.x-middle.x*cy.zoom()-pan.x,rendered.y-middle.y*cy.zoom()-pan.y)>.1)throw Error('Viewport sync lag');
   const box=owner.boundingBox({includeLabels:true}),labelBox=nodeLabel.boundingBox({includeLabels:true});
   if(labelBox.y1<=box.y2)throw Error('Node annotation overlaps its title');
   return {helpers:helpers.length,point:{...nodeLabel.renderedPosition()}};
  });
  // Clicking a native annotation selects its owner and still supports Space editing.
  await canvas.click({position:nativeCheck.point});await page.keyboard.press('Space');
  editor=page.getByRole('dialog',{name:'编辑拓扑备注'});await editor.waitFor();
  if(await editor.getByRole('textbox').inputValue()!=='数据库排查知识入口')throw Error('Annotation click lost owner');
  await editor.getByRole('textbox').press('Escape');
  await canvas.dblclick({position:nativeCheck.point});editor=page.getByRole('dialog',{name:'编辑拓扑备注'});await editor.waitFor();
  if(!await canvas.isVisible()||await editor.getByRole('textbox').inputValue()!=='数据库排查知识入口')throw Error('Annotation double click opened a note');
  await editor.getByRole('textbox').press('Escape');
  // Drag a real node with pointer events, checking its annotation at every pointer step.
  const dragStart=await canvas.evaluate(el=>({...el._cyreg.cy.getElementById('a').renderedPosition()})),canvasBox=await canvas.boundingBox();
  await page.mouse.move(canvasBox.x+dragStart.x,canvasBox.y+dragStart.y);await page.mouse.down();
  for(let i=1;i<=8;i++){
   await page.mouse.move(canvasBox.x+dragStart.x+i*5,canvasBox.y+dragStart.y+i*3);
   const gap=await canvas.evaluate(el=>{const cy=el._cyreg.cy,n=cy.getElementById('a'),label=cy.nodes('[kind="annotation"]').filter(a=>a.data('owner')==='a')[0];return Math.abs(n.position('x')-label.position('x'))});
   if(gap>.1)throw Error('Pointer drag annotation lagged');
  }
  await page.mouse.up();
  await page.screenshot({path:'docs/preview-canvas-graph-v0.12.5.png'});
  await page.reload();await page.getByRole('tab',{name:'知识拓扑',exact:true}).click();await canvas.waitFor();
  await page.waitForFunction(()=>document.querySelector('.graph-canvas')._cyreg.cy.nodes('[kind="annotation"]').some(n=>n.data('fullText')==='SQL排查的补充资料'));
  edgeId=await canvas.evaluate(el=>el._cyreg.cy.edges()[0].id());
  const longAnnotation='完整备注内容'.repeat(80);
  editor=await edit(edgeId,'edge',longAnnotation);await editor.getByRole('textbox').press('Enter');await editor.waitFor({state:'hidden'});
  await page.waitForFunction(text=>document.querySelector('.graph-canvas')._cyreg.cy.nodes('[kind="annotation"]').some(n=>n.data('fullText')===text),longAnnotation);
  await canvas.evaluate(el=>{const n=el._cyreg.cy.nodes('[kind="annotation"]').filter(n=>n.data('fullText').length>400)[0];if(n.data('label').split('\n').length>4||!n.data('label').endsWith('…'))throw Error('Long annotation summary not bounded');n.emit('mouseover')});
  if(await canvas.getAttribute('title')!==longAnnotation)throw Error('Full annotation hover lost text');
  editor=await edit(edgeId,'edge','');await editor.getByRole('textbox').press('Enter');await editor.waitFor({state:'hidden'});
  await page.waitForFunction(()=>!document.querySelector('.graph-canvas')._cyreg.cy.nodes('[kind="annotation"]').some(n=>n.data('fullText')==='SQL排查的补充资料'));
  await page.getByRole('button',{name:'手动关联管理',exact:true}).click();
  await canvas.evaluate(el=>{el._cyreg.cy.edges()[0].emit('tap')});
  if(await panel.locator('textarea').count())throw Error('Sidebar annotation input retained');
  await panel.getByRole('button',{name:'删除手动关联',exact:true}).click();await page.waitForFunction(()=>document.querySelector('.graph-canvas')._cyreg.cy.edges().length===0);
  await panel.locator('.n-select').last().click();await page.getByText('旅行计划 · 旅行计划.md',{exact:true}).click();await panel.getByRole('button',{name:'保存关联',exact:true}).click();
  await page.waitForFunction(()=>window.graphWrites.some(w=>w.source==='a'&&w.target==='c'&&w.confirmed));
  await page.getByRole('button',{name:'关闭知识拓扑',exact:true}).click();
  await page.locator('.tree-file').filter({hasText:'数据库排查'}).first().click();await page.getByRole('tab',{name:'知识拓扑',exact:true}).click();await canvas.waitFor();await canvas.evaluate(el=>{const cy=el._cyreg.cy;cy.getElementById('c').position({x:120,y:100});cy.zoom(1);cy.pan({x:150,y:120})});const notePoint=await canvas.evaluate(el=>({...el._cyreg.cy.getElementById('c').renderedPosition()}));await canvas.dblclick({position:notePoint});
  await page.waitForFunction(()=>!document.querySelector('.graph-main-overlay')||getComputedStyle(document.querySelector('.graph-main-overlay')).display==='none');
  if(!await page.locator('.markdown-preview').isVisible())throw Error('Double click note did not restore preview');
  await page.getByRole('tab',{name:'知识拓扑',exact:true}).click();await canvas.waitFor();
  await canvas.evaluate(el=>{const cy=el._cyreg.cy;cy.getElementById('c').position({x:120,y:100});cy.zoom(1);cy.pan({x:150,y:120})});
  const titlePoint=await canvas.evaluate(el=>{const n=el._cyreg.cy.getElementById('c'),b=n.renderedBoundingBox({includeNodes:false,includeLabels:true});return {x:(b.x1+b.x2)/2,y:b.y2-7}});
  await canvas.dblclick({position:titlePoint});
  await page.waitForFunction(()=>getComputedStyle(document.querySelector('.graph-main-overlay')).display==='none',null,{timeout:3000});
  if(!(await page.locator('.markdown-preview').innerText()).includes('旅行计划'))throw Error('Title double click did not open the correct note');
  async function reopenNote(){
   await page.getByRole('tab',{name:'知识拓扑',exact:true}).click();await canvas.waitFor();
   await canvas.evaluate(el=>{const cy=el._cyreg.cy;cy.getElementById('c').position({x:120,y:100});cy.zoom(1);cy.pan({x:150,y:120})});
  }
  await reopenNote();
  const titleClick=await canvas.evaluate(el=>{const n=el._cyreg.cy.getElementById('c'),b=n.renderedBoundingBox({includeLabels:true});return {x:n.renderedPosition('x'),y:b.y2-7}});
  await canvas.click({position:titleClick});await page.getByRole('button',{name:'打开选中笔记',exact:true}).waitFor();
  await page.keyboard.press('Space');editor=page.getByRole('dialog',{name:'编辑拓扑备注'});await editor.waitFor();
  if(await editor.getByRole('textbox').getAttribute('aria-label')!=='图上笔记备注')throw Error('Title selection lost Space editing');
  await editor.getByRole('textbox').press('Escape');await page.keyboard.press('Enter');
  await page.waitForFunction(()=>getComputedStyle(document.querySelector('.graph-main-overlay')).display==='none');
  if(!(await page.locator('.markdown-preview').innerText()).includes('旅行计划'))throw Error('Enter did not open selected note');
  await reopenNote();
  const cPoint=await canvas.evaluate(el=>({...el._cyreg.cy.getElementById('c').renderedPosition()}));
  await canvas.click({position:cPoint});await page.getByRole('button',{name:'打开选中笔记',exact:true}).click();
  await page.waitForFunction(()=>getComputedStyle(document.querySelector('.graph-main-overlay')).display==='none');
  if(!(await page.locator('.markdown-preview').innerText()).includes('旅行计划'))throw Error('Open button did not open selected note');
  await reopenNote();await canvas.click({position:cPoint});
  await page.getByRole('button',{name:'手动连线',exact:true}).click();await page.locator('.knowledge-graph').press('Enter');
  if(!await canvas.isVisible()||await page.getByRole('button',{name:'打开选中笔记',exact:true}).count())throw Error('Manual connect mode opened note');
  await page.getByRole('button',{name:'取消连线',exact:true}).click();
  // A note circle must still open its note when another annotation crosses it.
  await canvas.evaluate(el=>{const cy=el._cyreg.cy,label=cy.nodes('[kind="annotation"]').filter(n=>n.data('owner')==='a')[0];cy.getElementById('c').position({...label.position()});cy.style().selector('node[kind="note"]').style({'z-index-compare':'manual','z-index':0}).update()});
  const coveredPoint=await canvas.evaluate(el=>({...el._cyreg.cy.getElementById('c').renderedPosition()}));
  await canvas.dblclick({position:coveredPoint});
  await page.waitForFunction(()=>getComputedStyle(document.querySelector('.graph-main-overlay')).display==='none',null,{timeout:3000});
  if(!(await page.locator('.markdown-preview').innerText()).includes('旅行计划'))throw Error('Overlapping annotation intercepted note opening');
  await reopenNote();await canvas.click({position:cPoint});
  await page.screenshot({path:'docs/preview-graph-open-note-v0.12.5.png'});


  if(errors.length)throw Error(errors.join('\n'));
  console.log('PASS inline Space/Enter/Escape annotations, failed draft retry, emphasized titles, native same-frame node/edge annotations, line masking, real pointer drag, pan/zoom tracking, persistence, unchanged viewport/Markdown, manual relations, real point/title double click, Enter/button open, preserved Space/annotation double click');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
