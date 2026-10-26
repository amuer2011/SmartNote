const assert=require('node:assert/strict'),{chromium}=require('playwright');
(async()=>{const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});try{
 const context=await browser.newContext({viewport:{width:1660,height:1080}}),errors=[];
 await context.addInitScript(()=>{
  const state={activeId:'one',notes:[{id:'a',title:'第一篇笔记',path:'第一篇笔记.md',content:'# 第一篇笔记\n\n第一篇内容',hash:'ha',tags:[],project:'',updated:new Date().toISOString()},{id:'b',title:'第二篇笔记',path:'第二篇笔记.MD',content:'# 第二篇笔记\n\n第二篇内容',hash:'hb',tags:[],project:'',updated:new Date().toISOString()}],saves:[],chats:[],conversations:{}};
  let ui={fontSize:14,collapsedWorkspaces:[],entryOrder:{}};window.tabsTest=state;
  const workspaces=[{id:'one',name:'测试工作区',path:'D:/One'},{id:'two',name:'独立工作区',path:'D:/Two'}];
  window.go={main:{App:{
   Snapshot:async()=>({settings:{activeId:state.activeId,workspaces,ui,theme:'light',picGoUrl:''},notes:state.activeId==='one'?structuredClone(state.notes):[],directories:['保留.md'],tasks:[],attachments:[]}),
   DataDirectory:async()=>'D:/IsolatedTabsTest',SetUIPreferences:async value=>{ui=value;state.ui=value},GetBookmarks:async()=>[],Graph:async()=>[],GetGraphAnnotations:async()=>({nodes:{},relations:{}}),DueReminders:async()=>[],GetSyncStatus:async()=>({state:'idle'}),AutoArchive:async()=>null,
   GetHolidayYear:async year=>({year,holiday:{},type:{}}),GetLunarYear:async year=>({year,days:{}}),OpenWorkspace:async id=>{state.activeId=id},
   SaveNote:async input=>{const n=state.notes.find(n=>n.id===input.id);if(state.failSave)throw Error('CONFLICT');Object.assign(n,{content:input.content,hash:'saved'+Date.now()});state.saves.push(input);return structuredClone(n)},
   GetLLMProviders:async()=>({enabled:true,defaultId:'p',revision:'r',providers:[{id:'p',name:'测试模型服务',enabled:true,provider:'openai',endpoint:'https://example.test/v1',model:'chat-model',models:['chat-model'],selectedModels:['chat-model'],reasoningEffort:'',hasSecret:true,revision:'p1'}]}),
   GetLLMReasoningCapability:async()=>({known:true,efforts:[],message:''}),
   ListAssistantConversations:async()=>Object.values(state.conversations).map(c=>({id:c.id,title:c.title,updated:c.updated,messages:c.entries.length})),
   SaveAssistantConversation:async(workspace,c)=>{const result=structuredClone({...c,title:c.entries[0].content.slice(0,30),revision:'rev'+Date.now(),updated:new Date().toISOString()});state.conversations[c.id]=result;return result},
   GetAssistantConversation:async(workspace,id)=>structuredClone(state.conversations[id]),DeleteAssistantConversation:async(workspace,id)=>{delete state.conversations[id]},
   ChatLLM:async input=>{state.chats.push(JSON.parse(JSON.stringify(input)));return {text:'已阅读附件。',sources:[],proposals:[],steps:[]}},CancelLLM:async()=>{},
  }}};
 });
 const page=await context.newPage();page.on('pageerror',e=>errors.push(String(e)));await page.goto(process.env.SMARTNOTE_TEST_URL||'http://127.0.0.1:4175');await page.getByRole('button',{name:'切换工作区',exact:true}).waitFor();
 await page.evaluate(async()=>{const {useWorkspace}=await import('/src/stores/workspace.ts');window.testStore=useWorkspace()});
 const active=id=>page.waitForFunction(id=>window.testStore.active?.id===id,id);
 await page.locator('.tree-file').filter({hasText:'第一篇笔记'}).waitFor();assert.equal(await page.locator('.tree-file').filter({hasText:'.md'}).count(),0);assert(await page.locator('.tree-entry').filter({hasText:'保留.md'}).count());
 const first=page.locator('.tree-file').filter({hasText:'第一篇笔记'}),second=page.locator('.tree-file').filter({hasText:'第二篇笔记'}),directory=page.locator('.tree-entry').filter({hasText:'保留.md'});
 await first.click();await active('a');await second.click();await active('b');
 const tabs=page.getByRole('tablist',{name:'笔记标签页',exact:true});assert.equal(await tabs.getByRole('tab').count(),1);assert.equal(await tabs.locator('[data-tab-key="note:a"]').count(),0);
 await first.click({modifiers:['Control']});await active('a');assert.equal(await tabs.getByRole('tab').count(),2);
 await second.click({modifiers:['Meta']});await active('b');assert.equal(await tabs.getByRole('tab').count(),2);
 await first.click();await active('a');assert.equal(await tabs.getByRole('tab').count(),2);
 const selectedStyle=await first.evaluate(el=>getComputedStyle(el).backgroundColor),selection=await page.evaluate(()=>window.testStore.active.id);
 await second.hover();assert.notEqual(await second.evaluate(el=>getComputedStyle(el).backgroundColor),'rgba(0, 0, 0, 0)');assert.equal(await first.evaluate(el=>getComputedStyle(el).backgroundColor),selectedStyle);assert.equal(await page.evaluate(()=>window.testStore.active.id),selection);
 await directory.hover();assert.notEqual(await directory.evaluate(el=>getComputedStyle(el).backgroundColor),'rgba(0, 0, 0, 0)');assert.equal(await first.evaluate(el=>getComputedStyle(el).backgroundColor),selectedStyle);
 await second.click({button:'right'});const menu=page.getByRole('menu',{name:'文件操作',exact:true});await menu.waitFor();assert.equal(await menu.locator('.context-caption').count(),0);assert.equal(await menu.getByRole('menuitem',{name:'刷新工作区'}).count(),0);assert.equal(await menu.getByRole('menuitem',{name:'新建笔记'}).count(),0);assert.equal(await menu.getByRole('menuitem',{name:'新建文件夹'}).count(),0);await menu.getByRole('menuitem',{name:'打开',exact:true}).click();await active('b');assert.equal(await tabs.getByRole('tab').count(),2);
 await page.getByRole('button',{name:'关闭标签页 第一篇笔记',exact:true}).click();await first.click({button:'right'});await menu.getByRole('menuitem',{name:'在新标签页中打开',exact:true}).click();await active('a');assert.equal(await tabs.getByRole('tab').count(),2);
 await directory.click({button:'right'});assert(await menu.getByRole('menuitem',{name:'新建笔记'}).isVisible());assert(await menu.getByRole('menuitem',{name:'新建文件夹'}).isVisible());assert.equal(await menu.getByRole('menuitem',{name:'刷新工作区'}).count(),0);await page.keyboard.press('Escape');
 await page.getByRole('button',{name:'关闭标签页 第二篇笔记',exact:true}).click();await second.click({modifiers:['Control']});await active('b');
 await page.getByRole('button',{name:'新标签页',exact:true}).click();await page.locator('.new-tab-page h1').waitFor();assert.equal(await page.evaluate(()=>window.testStore.active),null);assert.equal(await tabs.getByRole('tab').count(),3);
 await tabs.locator('[data-tab-key="note:b"]').click();await active('b');await page.getByRole('button',{name:'关闭标签页 第二篇笔记',exact:true}).click();await page.locator('.new-tab-page h1').waitFor();assert.equal(await tabs.getByRole('tab').count(),2);
 await page.keyboard.press('Control+Shift+Tab');await active('a');await page.keyboard.press('Control+t');await page.locator('.new-tab-page h1').waitFor();
 await tabs.locator('[data-tab-key="note:a"]').click();await active('a');
 // Saving conflicts block switching and closing, preserving the draft.
 await page.evaluate(()=>{window.tabsTest.failSave=true;window.testStore.edit('# 第一篇笔记\n尚未保存的草稿')});await page.getByRole('button',{name:'新标签页',exact:true}).click();await page.waitForTimeout(120);assert.equal(await page.evaluate(()=>window.testStore.active.id),'a');assert((await page.evaluate(()=>window.testStore.content)).includes('尚未保存的草稿'));
 await page.getByRole('button',{name:'关闭标签页 第一篇笔记',exact:true}).click();assert.equal(await tabs.locator('[data-tab-key="note:a"]').count(),1);
 await page.evaluate(()=>{window.tabsTest.failSave=false;window.testStore.conflict=false;window.testStore.error=''});await page.keyboard.press('Control+s');await page.waitForFunction(()=>!window.testStore.dirty);
 // The note tab bar wraps inside the main area and tracks inspector resizing.
 await page.evaluate(async()=>{for(let i=0;i<14;i++)window.tabsTest.notes.push({id:'extra'+i,title:'布局验证笔记'+i,path:'布局验证笔记'+i+'.md',content:'# 布局验证',hash:'x'+i,tags:[],project:'',updated:new Date().toISOString()});await window.testStore.refresh()});
 for(let i=0;i<14;i++){await page.locator('.tree-file').filter({hasText:'布局验证笔记'+i}).filter({has:page.locator('span',{hasText:new RegExp('^布局验证笔记'+i+'$')})}).click({modifiers:[i%2?'Meta':'Control']});await active('extra'+i)}
 const measure=()=>page.evaluate(()=>{const bar=document.querySelector('.note-tabs-bar').getBoundingClientRect(),side=document.querySelector('.right-panel').getBoundingClientRect(),tabs=document.querySelector('.note-tabs'),rows=[...tabs.querySelectorAll('[role=tab]')].map(el=>el.getBoundingClientRect());return {right:bar.right,left:side.left,rows:new Set(rows.map(r=>Math.round(r.top))).size,overflow:tabs.scrollWidth-tabs.clientWidth,editor:document.querySelector('.editor-panel').getBoundingClientRect().height}});
 let layout=await measure();assert(layout.rows>1);assert(layout.right<=layout.left+1);assert(layout.overflow<=1);assert(layout.editor>150);
 await page.setViewportSize({width:1080,height:800});await page.waitForTimeout(150);layout=await measure();assert(layout.rows>1);assert(layout.right<=layout.left+1);assert(layout.overflow<=1);
 const divider=page.getByRole('separator',{name:'调整右侧栏宽度',exact:true});for(let i=0;i<8;i++)await divider.press('ArrowLeft');await page.waitForTimeout(150);layout=await measure();assert(layout.right<=layout.left+1);assert(layout.overflow<=1);
 const selectedVisible=await tabs.evaluate(el=>{const r=el.getBoundingClientRect(),s=el.querySelector('[aria-selected=true]').getBoundingClientRect();return s.top>=r.top-1&&s.bottom<=r.bottom+1});assert(selectedVisible);
 await page.screenshot({path:'docs/preview-note-navigation-v0.12.16.png',animations:'disabled'});await page.setViewportSize({width:1660,height:1080});
 await tabs.locator('[data-tab-key="note:a"]').click();await active('a');await page.evaluate(async()=>{window.tabsTest.notes=window.tabsTest.notes.filter(n=>!n.id.startsWith('extra'));await window.testStore.refresh()});
 // Right tabs default order, close/reopen and drag sorting.
 const right=page.getByRole('tablist',{name:'右侧栏页签',exact:true});assert.deepEqual(await right.getByRole('tab').evaluateAll(nodes=>nodes.map(n=>n.id)),['right-tab-outline','right-tab-related','right-tab-ai']);
 await page.getByRole('button',{name:'关闭关联',exact:true}).click();assert.equal(await page.locator('#right-tab-related').count(),0);
 await page.getByRole('button',{name:'面板布局',exact:true}).click();await page.getByRole('menuitemcheckbox',{name:'关联',exact:true}).click();await page.getByRole('button',{name:'面板布局',exact:true}).click();await page.locator('#right-tab-related').waitFor();
 const ai=page.locator('#right-tab-ai'),outline=page.locator('#right-tab-outline');await ai.dragTo(outline,{targetPosition:{x:2,y:15}});await page.waitForFunction(()=>window.tabsTest.ui.layout.rightTabOrder[0]==='ai');
 await ai.click();assert.equal(await page.locator('.ai-scope-picker,.ai-effort-trigger').count(),0);await page.getByRole('textbox',{name:'发送给 AI 的消息',exact:true}).waitFor();await page.waitForTimeout(200);
 const input=page.getByRole('textbox',{name:'发送给 AI 的消息',exact:true});
 await input.evaluate(node=>{const d=new DataTransfer();d.setData('text/plain','这是长文本资料。'.repeat(400));node.dispatchEvent(new ClipboardEvent('paste',{clipboardData:d,bubbles:true,cancelable:true}))});await page.locator('.ai-input-card .chat-attachment').waitFor();assert.equal(await input.inputValue(),'');
 await page.getByLabel('选择聊天附件',{exact:true}).setInputFiles([{name:'截图.png',mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aUj8AAAAASUVORK5CYII=','base64')},{name:'资料.pdf',mimeType:'application/pdf',buffer:Buffer.from('%PDF-1.4 sample')}]);
 await page.waitForFunction(()=>document.querySelectorAll('.ai-input-card .chat-attachment').length===3);await input.fill('结合附件说明');await page.getByRole('button',{name:'发送',exact:true}).click();await page.waitForFunction(()=>window.tabsTest.chats.length===1);
 assert.equal(await page.evaluate(()=>window.tabsTest.chats[0].messages.at(-1).attachments.length),3);assert.equal(await page.evaluate(()=>window.tabsTest.chats[0].scope),'workspace');await page.getByText('已阅读附件。',{exact:true}).waitFor();assert.equal(await page.locator('.ai-input-card .chat-attachment').count(),0);assert.equal(await page.locator('.ai-message.user .chat-attachment').count(),3);
 await page.screenshot({path:'docs/preview-note-tabs-chat-v0.12.16.png',animations:'disabled'});
 await page.getByRole('button',{name:'打开聊天历史',exact:true}).click();await page.getByRole('button',{name:'删除对话：结合附件说明',exact:true}).click();await page.getByRole('button',{name:'确认删除',exact:true}).click();await page.waitForFunction(()=>Object.keys(window.tabsTest.conversations).length===0);assert.equal(await page.locator('.ai-message .chat-attachment').count(),0);
 // Workspace changes discard old note tabs and attachment drafts.
 await page.evaluate(async()=>{await window.testStore.switchWorkspace('two')});await page.waitForFunction(()=>window.testStore.workspace.id==='two');assert.equal(await tabs.getByRole('tab').count(),1);assert.equal(await tabs.locator('[data-tab-key="note:a"]').count(),0);
 assert.deepEqual(errors,[]);console.log('PASS note tabs/default replacement/Ctrl and Meta new tab/context open/hover and persistent selection/wrapped main-area bounds/resizing and visible active tab/dedup/new page/close/keyboard/conflict; hidden md suffix; right tab defaults/reopen/drag; long pasted text/image/PDF chat payload/history deletion; workspace isolation');
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
