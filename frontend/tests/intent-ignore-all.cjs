const assert=require('node:assert/strict'),{chromium}=require('playwright');
(async()=>{const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});try{
 const page=await browser.newPage({viewport:{width:1440,height:960}}),errors=[];page.on('pageerror',e=>errors.push(String(e)));
 await page.addInitScript(()=>{
  let activeId='one',ui={fontSize:14,collapsedWorkspaces:[],entryOrder:{}};
  const workspaces=[{id:'one',name:'当前空间',path:'D:/One'},{id:'two',name:'另一空间',path:'D:/Two'}];
  const notes=Object.fromEntries(workspaces.map(w=>[w.id,Array.from({length:5},(_,i)=>({id:w.id+'-'+i,title:['明天提醒我开会','每周提醒我检查数据库','后天提醒我充值话费','已加入的任务','手动记录'][i],path:'Inbox/'+i+'.md',content:'# 记录'+i,hash:'h'+i,tags:[],project:'',updated:new Date().toISOString()}))]));
  const tasks=Object.fromEntries(workspaces.map(w=>[w.id,notes[w.id].map((n,i)=>({id:n.id+'_intent',noteId:n.id,noteTitle:n.title,title:n.title,line:0,inferred:i<4,sourceRevision:'v1',needsConfirmation:i<2,candidate:i<2,done:false,dueAt:'',priority:'normal',tags:[]}))]));
  const calls=[],ignored=new Set();window.batchTest={notes,tasks,calls,ignored,fail:false,slow:false,failSave:false};
  window.go={main:{App:{
   Snapshot:async()=>({settings:{workspaces,activeId,ui,theme:'light',picGoUrl:''},notes:notes[activeId].map(n=>({...n,automationIgnored:ignored.has(n.id)})),directories:['Inbox'],tasks:tasks[activeId].filter(t=>!ignored.has(t.noteId)),attachments:[]}),
   DataDirectory:async()=> 'D:/TestData',SetUIPreferences:async v=>{ui=v},GetBookmarks:async()=>[],Graph:async()=>[],GetGraphAnnotations:async()=>({nodes:{},relations:{}}),DueReminders:async()=>[],GetSyncStatus:async()=>({state:'idle'}),AutoArchive:async()=>null,OpenWorkspace:async id=>{activeId=id},
   GetHolidayYear:async year=>({year,holiday:{},type:{}}),GetLunarYear:async year=>({year,days:{}}),
   SaveNote:async input=>{if(window.batchTest.failSave)throw Error('CONFLICT');const n=notes[activeId].find(n=>n.id===input.id);Object.assign(n,{content:input.content,hash:'saved'});return structuredClone(n)},
   IgnoreAllTaskIntents:async input=>{calls.push(structuredClone(input));if(window.batchTest.slow)await new Promise(r=>window.releaseBatch=r);if(input.workspaceId!==activeId)throw Error('工作区已切换');if(window.batchTest.fail)throw Error('写入失败，请重试');for(const item of input.tasks){const t=tasks[activeId].find(t=>t.id===item.taskId);if(!t?.needsConfirmation||t.sourceRevision!==item.sourceRevision)throw Error('STALE')}for(const item of input.tasks)ignored.add(tasks[activeId].find(t=>t.id===item.taskId).noteId);return input.tasks.length},
  }}};
 });
 await page.goto(process.env.SMARTNOTE_TEST_URL||'http://127.0.0.1:4175');
 await page.evaluate(async()=>{const {useWorkspace}=await import('/src/stores/workspace.ts');window.testStore=useWorkspace()});
 const modal=page.getByRole('dialog').filter({has:page.getByRole('button',{name:'忽略所有',exact:true})}),button=()=>page.getByRole('button',{name:'忽略所有',exact:true});
 await button().waitFor();
 assert(await modal.locator('.intent-actions').evaluate(el=>{const labels=[...el.querySelectorAll('button')].map(b=>b.textContent.trim());return labels.indexOf('忽略所有')===labels.indexOf('忽略此笔记')+1}));
 // Previously deferred results also belong to the current workspace's pending set.
 await page.getByRole('button',{name:'稍后确认',exact:true}).click();await modal.waitFor({state:'hidden'});
 await page.evaluate(async()=>{window.batchTest.tasks.one[2].needsConfirmation=true;window.batchTest.tasks.one[2].candidate=true;await window.testStore.refresh()});
 await button().waitFor();assert((await modal.innerText()).includes('全部 3 篇待确认笔记'));
 await page.waitForTimeout(350);await page.screenshot({path:'docs/preview-intent-ignore-all-v0.12.12.png',animations:'disabled'});
 await page.evaluate(()=>window.batchTest.fail=true);await button().click();await modal.getByRole('alert').filter({hasText:'写入失败'}).waitFor();
 assert.equal(await page.evaluate(()=>window.batchTest.ignored.size),0);assert.equal(await page.evaluate(()=>window.batchTest.calls.at(-1).tasks.length),3);
 // Saving conflicts must stop the batch before any backend call.
 await page.evaluate(async()=>{await window.testStore.select(window.batchTest.notes.one[4]);window.batchTest.failSave=true;window.testStore.edit(window.testStore.content+'\n草稿')});
 const before=await page.evaluate(()=>window.batchTest.calls.length);await button().click();await modal.getByRole('alert').filter({hasText:'未保存'}).waitFor();assert.equal(await page.evaluate(()=>window.batchTest.calls.length),before);
 await page.evaluate(async()=>{window.batchTest.failSave=false;window.testStore.conflict=false;await window.testStore.save();window.batchTest.fail=false;window.batchTest.slow=true});
 await button().click();await page.waitForFunction(()=>!!window.releaseBatch);assert(await button().isDisabled());assert(await page.getByRole('button',{name:'忽略此笔记',exact:true}).isDisabled());
 await page.evaluate(()=>window.releaseBatch());await modal.waitFor({state:'hidden'});
 assert.deepEqual(await page.evaluate(()=>window.testStore.data.tasks.map(t=>t.noteId)),['one-3','one-4']);assert.equal(await page.evaluate(()=>window.batchTest.ignored.size),3);
 assert(await page.evaluate(()=>window.testStore.data.notes.filter(n=>n.automationIgnored).length===3));
 assert.equal(await page.evaluate(()=>window.batchTest.calls.at(-1).workspaceId),'one');
 assert(await page.evaluate(()=>window.batchTest.notes.one[0].content==='# 记录0'));
 // New recognitions are not disabled globally; a late failure stays in its own workspace.
 await page.evaluate(async()=>{window.batchTest.tasks.one[4].inferred=true;window.batchTest.tasks.one[4].needsConfirmation=true;await window.testStore.refresh()});await button().waitFor();
 await button().click();await page.waitForFunction(()=>window.batchTest.calls.length===3);
 await page.evaluate(async()=>{await window.testStore.switchWorkspace('two');window.releaseBatch()});
 await button().waitFor();await page.waitForTimeout(150);assert.equal(await modal.getByRole('alert').count(),0);
 assert.equal(await page.evaluate(()=>window.testStore.workspace.id),'two');assert.equal(await page.evaluate(()=>window.testStore.data.tasks.filter(t=>t.needsConfirmation).length),2);
 await page.evaluate(()=>window.batchTest.slow=false);await button().click();await modal.waitFor({state:'hidden'});assert.equal(await page.evaluate(()=>window.batchTest.calls.at(-1).workspaceId),'two');
 assert.deepEqual(errors,[]);console.log('PASS ignore-all adjacency/count/deferred queue; atomic API selection/current workspace; failure/save conflict/retry/busy guard; accepted/manual tasks and notes preserved; future recognition and late workspace errors isolated');
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
