const assert=require('node:assert/strict'),{chromium}=require('playwright');
(async()=>{const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});try{
 const context=await browser.newContext({viewport:{width:1440,height:960}}),errors=[];
 await context.addInitScript(()=>{
  const initial=()=>({activeId:'one',notes:{one:[{id:'a',title:'上次打开的笔记',path:'项目/上次打开的笔记.md',content:'# 上次打开的笔记\n内容',hash:'ha',tags:[],project:'',updated:new Date().toISOString()},{id:'c',title:'手动选择的笔记',path:'手动选择的笔记.md',content:'# 手动选择的笔记',hash:'hc',tags:[],project:'',updated:new Date().toISOString()}],two:[{id:'b',title:'第二工作区笔记',path:'资料/第二工作区笔记.md',content:'# 第二工作区笔记',hash:'hb',tags:[],project:'',updated:new Date().toISOString()}]}});
  const state=JSON.parse(localStorage.getItem('session-test-backend')||'null')||initial();let ui={fontSize:14,collapsedWorkspaces:[],entryOrder:{}};
  const persist=()=>localStorage.setItem('session-test-backend',JSON.stringify(state));persist();window.sessionTest={state,persist,saves:0};
  const workspaces=[{id:'one',name:'空间一',path:'D:/One'},{id:'two',name:'空间二',path:'D:/Two'}];
  window.go={main:{App:{
   Snapshot:async()=>{if(localStorage.getItem('session-test-fail-snapshot'))throw Error('Snapshot unavailable');return {settings:{activeId:state.activeId,workspaces,ui,theme:'light',picGoUrl:''},notes:structuredClone(state.notes[state.activeId]),directories:[],tasks:[],attachments:[]}},
   DataDirectory:async()=>{if(localStorage.getItem('session-test-slow-directory'))await new Promise(r=>window.releaseDirectory=r);return localStorage.getItem('session-test-directory')||'D:/ProfileOne'},
   SetUIPreferences:async value=>{ui=value},GetBookmarks:async()=>[],Graph:async()=>[],GetGraphAnnotations:async()=>({nodes:{},relations:{}}),DueReminders:async()=>[],GetSyncStatus:async()=>({state:'idle'}),AutoArchive:async()=>null,
   GetHolidayYear:async year=>({year,holiday:{},type:{}}),GetLunarYear:async year=>({year,days:{}}),OpenWorkspace:async id=>{state.activeId=id;persist()},
   SaveNote:async input=>{window.sessionTest.saves++;const note=state.notes[state.activeId].find(n=>n.id===input.id);if(input.hash!==note.hash)throw Error('CONFLICT');Object.assign(note,{content:input.content,hash:'saved'});persist();return structuredClone(note)},
  }}};
 });
 let page;
 async function launch(){
  if(page)await page.close();page=await context.newPage();page.on('pageerror',e=>errors.push(String(e)));
  await page.goto(process.env.SMARTNOTE_TEST_URL||'http://127.0.0.1:4175');await page.getByRole('button',{name:'切换工作区',exact:true}).waitFor();
  await page.evaluate(async()=>{const {useWorkspace}=await import('/src/stores/workspace.ts');window.testStore=useWorkspace()});
  await page.waitForFunction(()=>!!window.testStore.workspace);await page.waitForTimeout(120);
 }
 async function open(title){await page.locator('.note-card').filter({has:page.getByRole('heading',{name:title,exact:true})}).click();await page.locator('.editor-panel .note-header-meta strong').filter({hasText:title}).waitFor()}
 async function expectActive(id){await page.waitForFunction(id=>window.testStore.active?.id===id,id)}
 async function switchTo(id){await page.getByRole('button',{name:'切换工作区',exact:true}).click();await page.locator('[data-workspace-id="'+id+'"]').click();await page.waitForFunction(id=>window.testStore.workspace?.id===id,id)}
 const sessionKey=(directory,id)=>'smartnote:last-open-note:'+JSON.stringify([directory,id]);
 await launch();assert.equal(await page.evaluate(()=>window.testStore.active),null);await open('上次打开的笔记');
 assert.equal(await page.evaluate(key=>JSON.parse(localStorage.getItem(key)).id,sessionKey('D:/ProfileOne','one')),'a');
 // A new page in the same persistent context simulates closing/reopening WebView2.
 await launch();await expectActive('a');await page.locator('.markdown-preview h1').filter({hasText:'上次打开的笔记'}).waitFor();assert.equal(await page.locator('.source-pane').count(),0);assert.equal(await page.evaluate(()=>window.sessionTest.saves),0);
 await page.waitForTimeout(300);await page.screenshot({path:'docs/preview-note-session-v0.12.13.png',animations:'disabled'});
 await switchTo('two');assert.equal(await page.evaluate(()=>window.testStore.active),null);await open('第二工作区笔记');await launch();await expectActive('b');
 await switchTo('one');await launch();await expectActive('a');
 // Stable IDs find notes after moving; older path references still locate the file.
 await page.evaluate(()=>{window.sessionTest.state.notes.one[0].path='归档/已移动.md';window.sessionTest.persist()});await launch();await expectActive('a');assert.equal(await page.evaluate(()=>window.testStore.active.path),'归档/已移动.md');
 await page.evaluate(()=>{window.sessionTest.state.notes.one[0].id='new-a';window.sessionTest.persist()});await launch();await expectActive('new-a');
 // Explicitly closing the document must keep it closed on the next launch.
 await page.keyboard.press('Control+w');await page.waitForFunction(()=>!window.testStore.active);await launch();assert.equal(await page.evaluate(()=>window.testStore.active),null);
 await open('上次打开的笔记');await page.evaluate(()=>{window.sessionTest.state.notes.one=window.sessionTest.state.notes.one.filter(n=>n.id!=='new-a');window.sessionTest.persist()});
 await launch();assert.equal(await page.evaluate(()=>window.testStore.active),null);assert.equal(await page.evaluate(key=>localStorage.getItem(key),sessionKey('D:/ProfileOne','one')),null);
 await open('手动选择的笔记');
 // The application data directory isolates records even when workspace IDs match.
 await page.evaluate(()=>localStorage.setItem('session-test-directory','D:/OtherProfile'));await launch();assert.equal(await page.evaluate(()=>window.testStore.active),null);
 await page.evaluate(()=>localStorage.removeItem('session-test-directory'));await launch();await expectActive('c');
 // Existing draft recovery and external-change conflict protection still run.
 await page.evaluate(()=>{const n=window.sessionTest.state.notes.one[0];localStorage.setItem('smartnote:draft:one:c',JSON.stringify({content:n.content+'\n未保存草稿',hash:n.hash}));n.hash='external';n.content='# 外部修改';window.sessionTest.persist()});
 await launch();await expectActive('c');assert(await page.evaluate(()=>window.testStore.conflict));assert((await page.evaluate(()=>window.testStore.content)).includes('未保存草稿'));assert.equal(await page.evaluate(()=>window.sessionTest.saves),0);
 await page.evaluate(()=>localStorage.removeItem('smartnote:draft:one:c'));
 // An early user selection wins over a slow startup restoration.
 await page.evaluate(()=>{const n={...window.sessionTest.state.notes.one[0],id:'early',title:'启动时选择',path:'启动时选择.md',content:'# 启动时选择',hash:'early'};window.sessionTest.state.notes.one.push(n);window.sessionTest.persist();localStorage.setItem('session-test-slow-directory','true')});
 await launch();await page.waitForFunction(()=>!!window.releaseDirectory);await open('启动时选择');await page.evaluate(()=>{localStorage.removeItem('session-test-slow-directory');window.releaseDirectory()});await expectActive('early');await page.waitForTimeout(150);assert.equal(await page.evaluate(()=>window.testStore.active.id),'early');
 // Corrupt browser data must not block opening notes.
 await page.evaluate(key=>localStorage.setItem(key,'{broken'),sessionKey('D:/ProfileOne','one'));await launch();assert.equal(await page.evaluate(()=>window.testStore.active),null);await open('启动时选择');await launch();await expectActive('early');
 assert.deepEqual(errors,[]);console.log('PASS last-note persistence across fresh pages; workspace/profile isolation; moved ID/path fallback; explicit close/deletion; preview and draft conflict preservation; slow startup user selection and corrupt storage');
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
