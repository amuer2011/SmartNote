const assert=require('node:assert/strict'),{chromium}=require('playwright');
(async()=>{const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});try{
 const page=await browser.newPage({viewport:{width:1440,height:920}}),errors=[];page.on('pageerror',e=>errors.push(String(e)));
 await page.addInitScript(()=>{
  const workspaces=Array.from({length:14},(_,i)=>({id:'w'+(i+1),name:'知识空间'+(i+1),path:'D:/Notes/Workspace'+(i+1)}));
  let activeId='w1',ui={fontSize:14,collapsedWorkspaces:[],entryOrder:{}},version=0;
  const today=new Date(),date=[today.getFullYear(),String(today.getMonth()+1).padStart(2,'0'),String(today.getDate()).padStart(2,'0')].join('-');
  const notes=Object.fromEntries(workspaces.map((w,i)=>[w.id,[{id:w.id+'-note',title:'笔记'+(i+1),path:'独立目录'+(i+1)+'/笔记'+(i+1)+'.md',content:'# 笔记'+(i+1)+'\n\n正文关键词'+(i+1),hash:'h'+i,tags:[],project:'',updated:today.toISOString()}]]));
  const chats=Object.fromEntries(workspaces.map(w=>[w.id,[]]));
  window.testState={notes,chats,calls:[],get activeId(){return activeId}};
  const snapshot=()=>({settings:{workspaces,activeId,ui,theme:'light',picGoUrl:''},notes:structuredClone(notes[activeId]),directories:['独立目录'+activeId.slice(1)],tasks:[{id:activeId+'-task',noteId:activeId+'-note',noteTitle:'笔记'+activeId.slice(1),title:'待办'+activeId.slice(1),done:false,candidate:false,dueAt:date,priority:'normal',project:'',tags:[],line:0}],attachments:[{id:activeId+'-asset',name:'附件'+activeId.slice(1)+'.txt',size:10,mime:'text/plain',created:today.toISOString()}]});
  const provider={id:'p',name:'测试供应商',provider:'openai',endpoint:'https://example.com/v1',model:'gpt-4.1-mini',models:['gpt-4.1-mini'],selectedModels:[],reasoningEffort:''};
  window.go={main:{App:{
   Snapshot:async()=>{const value=snapshot();if(window.slowNextSnapshot){window.slowNextSnapshot=false;await new Promise(r=>window.releaseSnapshot=r)}if(window.failNextSnapshot){window.failNextSnapshot=false;throw Error('snapshot failed')}return value},
   OpenWorkspace:async id=>{window.testState.calls.push(['open',id]);if(window.failOpen){window.failOpen=false;throw Error('open failed')}activeId=id},
   ChooseWorkspace:async()=>{if(window.cancelChoose)throw Error('已取消');const w={id:'new',name:'新空间',path:'D:/New'};workspaces.push(w);notes.new=[];chats.new=[];activeId=w.id;return w},
   RenameWorkspace:async(id,name)=>{workspaces.find(w=>w.id===id).name=name},
   DataDirectory:async()=> 'D:/IsolatedTest',SetUIPreferences:async v=>{ui=v},SetPreferences:async()=>{},
   ListWorkspaceFiles:async id=>{window.testState.calls.push(['list',id]);return {notes:notes[id],directories:[]}},
   SaveNote:async input=>{if(window.failSave)throw Error('CONFLICT');const n=notes[activeId].find(n=>n.id===input.id);if(!n)throw Error('wrong workspace');Object.assign(n,{content:input.content,hash:'saved'+(++version)});return structuredClone(n)},
   AutoArchive:async()=>null,Graph:async()=>[],GetGraphAnnotations:async()=>({revision:'',nodes:{},relations:{}}),
   GetBookmarks:async id=>[{id:id+'-bookmark',noteId:id+'-note',kind:'note',label:'书签'+id.slice(1),line:0,column:0}],
   DueReminders:async()=>[],GetSyncStatus:async()=>({state:'idle'}),
   GetHolidayYear:async year=>({year,holiday:{},type:{},complete:true}),GetLunarYear:async year=>({year,days:{}}),
   GetLLMProviders:async()=>({enabled:true,defaultId:'p',providers:[provider]}),GetLLMReasoningOptions:async()=>({options:[],supported:false}),
   ListAssistantConversations:async id=>chats[id].map(c=>({id:c.id,title:c.title,updated:c.updated,messages:c.entries.length})),
   GetAssistantConversation:async(id,key)=>structuredClone(chats[id].find(c=>c.id===key)),
   SaveAssistantConversation:async(id,c)=>{const saved={...c,revision:String(++version),title:'对话'+id,updated:today.toISOString()};const i=chats[id].findIndex(x=>x.id===c.id);if(i<0)chats[id].push(saved);else chats[id][i]=saved;return structuredClone(saved)},
   ChatLLM:async input=>{window.testState.calls.push(['chat',input.workspaceId,input.messages.at(-1).content]);const text='回复'+input.workspaceId;if(window.delayChat)await new Promise(r=>window.releaseChat=r);return {text,sources:[],proposals:[],steps:[]}},CancelLLM:async()=>{},
   SemanticSearch:async text=>{const value={results:notes[activeId].map(note=>({note:structuredClone(note),snippet:'检索'+activeId,line:0,score:.9})),mode:'hybrid',message:''};if(window.slowSearch)await new Promise(r=>window.releaseSearch=r);return value},
   AttachmentUsage:async()=>[],GetTaskArchive:async()=>[],ListTrash:async id=>[{source:'notes',path:'batch',name:'batch',directory:true},{source:'notes',path:'batch/Inbox',name:'Inbox',directory:true},{source:'notes',path:'batch/Inbox/回收'+id+'.md',name:'回收'+id+'.md',directory:false}].map(e=>({...e,size:100,modified:today.toISOString()})),ReadTrashFile:async id=>{if(window.slowTrash)await new Promise(r=>window.releaseTrash=r);return '# 回收正文'+id},GetAssistantMemory:async()=>({revision:'',text:''}),
  }}};
 });
 await page.goto(process.env.SMARTNOTE_TEST_URL||'http://127.0.0.1:4175');
 await page.getByRole('button',{name:'切换工作区',exact:true}).waitFor();
 await page.evaluate(async()=>{const {useWorkspace}=await import('/src/stores/workspace.ts');window.testStore=useWorkspace()});
 const picker=page.getByRole('button',{name:'切换工作区',exact:true}),menu=page.getByRole('menu',{name:'已打开的工作区'});
 async function switchTo(id){await picker.click();if(Number(id.slice(1))>5)await page.getByRole('menuitem',{name:'展示全部工作区'}).click();await menu.locator('[data-workspace-id="'+id+'"]').click();await page.waitForFunction(id=>window.testStore.workspace?.id===id,id);await page.waitForTimeout(100)}
 async function treeOpen(){await page.getByRole('tab',{name:'导航',exact:true}).click();await page.getByRole('button',{name:'展开全部目录',exact:true}).click();}
 assert.equal(await page.locator('#sidebar-panel-workspaces').count(),0);
 await picker.click();assert.equal(await menu.getByRole('menuitemradio').count(),5);assert.equal(await menu.getByRole('menuitemradio',{checked:true}).count(),1);
 await page.getByRole('menuitem',{name:'展示全部工作区'}).click();assert.equal(await menu.getByRole('menuitemradio').count(),14);
 const scroll=await menu.locator('.workspace-menu-list').evaluate(el=>({scroll:el.scrollHeight,height:el.clientHeight}));assert(scroll.scroll>scroll.height);
 const bounds=await menu.boundingBox(),footer=await menu.locator('footer').boundingBox();assert(bounds.y+bounds.height<=920);assert(footer.y+footer.height<=920);assert(await page.getByRole('menuitem',{name:'新建工作区',exact:true}).isVisible());
 await page.screenshot({path:'docs/preview-workspace-picker-v0.12.9.png'});
 await page.keyboard.press('Escape');assert.equal(await menu.count(),0);assert(await picker.evaluate(el=>el===document.activeElement));
 await picker.click();assert.equal(await menu.getByRole('menuitemradio').count(),5);await menu.getByRole('menuitemradio').first().press('ArrowDown');assert.equal(await page.evaluate(()=>document.activeElement.dataset.workspaceId),'w2');await page.keyboard.press('Enter');await page.waitForFunction(()=>window.testStore.workspace?.id==='w2');
 await switchTo('w8');await treeOpen();assert.equal(await page.locator('.workspace-root').count(),1);assert.equal(await page.locator('.tree-file').count(),1);assert((await page.locator('.tree-file').innerText()).includes('笔记8'));assert.equal(await page.evaluate(()=>window.testState.calls.filter(c=>c[0]==='list').length),0);
 await page.locator('.tree-file').click();await page.locator('.markdown-preview h1').filter({hasText:'笔记8'}).waitFor();
 await page.getByRole('tab',{name:'待办事项',exact:true}).click();await page.getByRole('textbox',{name:'搜索待办',exact:true}).fill('不应保留');await switchTo('w2');await page.getByRole('tab',{name:'待办事项',exact:true}).click();assert.equal(await page.getByRole('textbox',{name:'搜索待办',exact:true}).inputValue(),'');assert((await page.locator('.schedule-list').innerText()).includes('待办2'));assert(!(await page.locator('.schedule-list').innerText()).includes('待办8'));
 await page.getByRole('tab',{name:'书签',exact:true}).click();assert((await page.locator('.bookmark-list').innerText()).includes('笔记2'));assert(!(await page.locator('.bookmark-list').innerText()).includes('笔记8'));
 await page.getByRole('tab',{name:'知识拓扑',exact:true}).click();await page.locator('.graph-canvas').waitFor();await page.waitForFunction(()=>document.querySelector('.graph-canvas')._cyreg?.cy?.nodes().length===1);assert.deepEqual(await page.locator('.graph-canvas').evaluate(el=>el._cyreg.cy.nodes().map(n=>n.id())),['w2-note']);await page.keyboard.press('Escape');
 await page.getByRole('tab',{name:'附件库',exact:true}).click();assert((await page.locator('#sidebar-panel-attachments').innerText()).includes('附件2.txt'));assert(!(await page.locator('#sidebar-panel-attachments').innerText()).includes('附件8.txt'));
 // Old bridge snapshots cannot overwrite the newly selected workspace.
 await page.evaluate(()=>{window.slowNextSnapshot=true;window.refreshPromise=window.testStore.refresh()});await page.waitForFunction(()=>!!window.releaseSnapshot);await switchTo('w8');await page.evaluate(async()=>{window.releaseSnapshot();await window.refreshPromise});assert.equal(await page.evaluate(()=>window.testStore.workspace.id),'w8');assert.deepEqual(await page.evaluate(()=>window.testStore.data.notes.map(n=>n.id)),['w8-note']);
 // A previous workspace's semantic search must not repopulate the new panel.
 await page.evaluate(()=>window.slowSearch=true);await page.getByRole('tab',{name:'搜索',exact:true}).click();await page.locator('.search-query input').fill('关键词');await page.waitForFunction(()=>!!window.releaseSearch);await switchTo('w2');await page.evaluate(()=>window.releaseSearch());await page.waitForTimeout(200);assert.equal(await page.evaluate(()=>window.testStore.workspace.id),'w2');await page.getByRole('tab',{name:'搜索',exact:true}).click();assert.equal(await page.locator('.search-query input').inputValue(),'');assert.equal(await page.locator('.search-result').count(),0);await switchTo('w8');
 // A recycled note read started before a switch cannot reappear in the main pane.
 await page.evaluate(()=>window.slowTrash=true);await page.getByRole('tab',{name:'回收站',exact:true}).click();await page.locator('.trash-entry').filter({hasText:'Inbox'}).click();await page.locator('.trash-entry').filter({hasText:'回收w8.md'}).click();await page.waitForFunction(()=>!!window.releaseTrash);await switchTo('w2');await page.evaluate(()=>{window.slowTrash=false;window.releaseTrash()});await page.getByRole('tab',{name:'回收站',exact:true}).click();assert(!(await page.getByRole('region',{name:'回收笔记预览'}).innerText()).includes('回收正文w8'));await page.locator('.trash-entry').filter({hasText:'Inbox'}).click();await page.locator('.trash-entry').filter({hasText:'回收w2.md'}).click();await page.getByRole('region',{name:'回收笔记预览'}).getByRole('heading',{name:'回收正文w2',exact:true}).waitFor();await switchTo('w8');
 // Chat is saved and restored per workspace; late replies never enter a new chat.
 await treeOpen();await page.locator('.tree-file').click();await page.getByRole('tab',{name:'AI 聊天',exact:true}).click();
 const input=page.getByRole('textbox',{name:'发送给 AI 的消息',exact:true});await input.waitFor();await input.fill('只属于空间8的问题');await input.press('Control+Enter');await page.getByRole('log',{name:'AI 对话记录'}).getByText('回复w8',{exact:true}).waitFor();
 await switchTo('w2');await treeOpen();await page.locator('.tree-file').click();await page.getByRole('tab',{name:'AI 聊天',exact:true}).click();assert(!(await page.getByRole('log',{name:'AI 对话记录'}).innerText()).includes('只属于空间8'));
 await page.evaluate(()=>window.delayChat=true);await input.fill('只属于空间2的问题');await input.press('Control+Enter');await page.waitForFunction(()=>!!window.releaseChat);await switchTo('w8');await treeOpen();await page.locator('.tree-file').click();await page.getByRole('tab',{name:'AI 聊天',exact:true}).click();await page.evaluate(()=>window.releaseChat());await page.waitForTimeout(400);const history=await page.getByRole('log',{name:'AI 对话记录'}).innerText();assert(history.includes('只属于空间8的问题')&&history.includes('回复w8'));assert(!history.includes('只属于空间2的问题')&&!history.includes('回复w2'));
 // Failed switches and cancelled creation preserve the current note.
 await page.evaluate(()=>window.failOpen=true);await switchToFailed('w1');assert.equal(await page.evaluate(()=>window.testStore.workspace.id),'w8');assert.equal(await page.evaluate(()=>window.testStore.active.id),'w8-note');
 await page.evaluate(()=>{window.cancelChoose=true});await picker.click();await page.getByRole('menuitem',{name:'新建工作区',exact:true}).click();await page.waitForFunction(()=>!window.testStore.switching);assert.equal(await page.evaluate(()=>window.testStore.active.id),'w8-note');
 await page.evaluate(()=>{window.failSave=true;window.testStore.edit(window.testStore.content+'\n未保存草稿')});await switchToFailed('w1');assert.equal(await page.evaluate(()=>window.testStore.workspace.id),'w8');assert((await page.evaluate(()=>window.testStore.content)).includes('未保存草稿'));
 await page.evaluate(()=>{window.failSave=false;window.testStore.conflict=false});await page.keyboard.press('Control+s');await page.waitForFunction(()=>!window.testStore.dirty);
 await page.evaluate(()=>window.failNextSnapshot=true);await switchToFailed('w1');assert.equal(await page.evaluate(()=>window.testState.activeId),'w8');assert.equal(await page.evaluate(()=>window.testStore.workspace.id),'w8');
 await page.setViewportSize({width:900,height:540});await picker.click();await page.getByRole('menuitem',{name:'展示全部工作区'}).click();const small=await menu.boundingBox();assert(small.y+small.height<=540);assert(await menu.getByRole('menuitem',{name:'新建工作区',exact:true}).isVisible());await page.keyboard.press('Escape');
 await page.evaluate(()=>{window.cancelChoose=false});await picker.click();await menu.getByRole('menuitem',{name:'新建工作区',exact:true}).click();await page.waitForFunction(()=>window.testStore.workspace?.id==='new');assert.equal(await page.locator('.workspace-root').count(),1);assert.equal(await page.evaluate(()=>window.testStore.data.tasks[0].id),'new-task');
 assert.deepEqual(errors,[]);console.log('PASS: workspace picker first five/expand/scroll/fixed footer/keyboard/small screen; current-only navigation/tasks/bookmarks/graph/attachments/chat; late snapshot and reply rejection; chat persistence; save conflict/open failure/snapshot rollback/cancelled and successful creation');
 async function switchToFailed(id){await picker.click();await menu.locator('[data-workspace-id="'+id+'"]').click();await page.waitForFunction(()=>!window.testStore.switching);await page.waitForTimeout(100)}
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
