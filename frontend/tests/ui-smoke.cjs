const { chromium } = require('playwright');
const path = require('path');
const baseUrl = process.env.SMARTNOTE_TEST_URL || 'http://127.0.0.1:4173/';
process.chdir(path.resolve(__dirname, '../..'));
(async () => {
 const browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || (process.platform === 'win32' ? 'C:/Program Files/Google/Chrome/Application/chrome.exe' : undefined), headless: true, args: ['--disable-gpu'] });
 const page = await browser.newPage({ viewport: { width: 1440, height: 920 }, deviceScaleFactor: 1 });
 const errors = []; page.on('pageerror', e => errors.push(String(e)));
 await page.goto(baseUrl); await page.waitForSelector('.welcome-panel');
 await page.screenshot({ path: 'docs/preview-empty.png' });
 for (const label of ['待办事项', '搜索', '知识拓扑', '附件库']) {
  await page.locator('.main-nav button').filter({ hasText: label }).click();
  await page.waitForTimeout(100);
 }
 await page.reload();
 await page.addInitScript(() => {
  let note = { id: 'test-note-1', path: '技术知识/构建本地优先的知识库.md', title: '构建本地优先的知识库', content: '---\nid: test-note-1\ntitle: 构建本地优先的知识库\ntags: [产品设计, Markdown]\nproject: SmartNote\n---\n\n# 构建本地优先的知识库\n\n好的工具，让我们把注意力留给思考本身。\n\n## 为什么选择本地优先\n\n笔记以标准 Markdown 文件保存，随时可以用喜欢的编辑器打开。\n\n> 记录当下，连接知识。数据始终属于你。\n\n## 下一步行动\n\n- [ ] 完善附件备份流程\n- [ ] 整理项目实施计划\n\n## 一个简单的约定\n\n```go\n// Markdown is the source of truth.\nfunc SaveNote(note Note) error {\n    return atomicWrite(note.Path, note.Content)\n}\n```\n\n明天需要整理离线检索的测试资料。\n', hash: 'initial-hash', tags: ['产品设计','Markdown'], project: 'SmartNote', updated: new Date().toISOString() };
  let semanticConfig={revision:'',model:'BAAI/bge-small-zh-v1.5',automatic:true,automaticVersion:1,threshold:0.55,margin:0.04,categories:[{id:'db',name:'数据库',description:'SQL数据库排查',directory:'技术/数据库',enabled:true}]};
  let archiveConfig={revision:'',rules:[]};const archiveUndo=new Map();
  let confirmed = false;
  let details = { dueAt: '', priority: 'normal' };
  const created = [];
  let directories=['空文件夹','空文件夹/子目录'];
  const directoryFixtures = [
   ['根层文件', '根层文件.md'], ['父层文件', '层级测试/父层文件.md'],
   ['子层文件', '层级测试/子目录/子层文件.md'], ['相似前缀文件', '层级测试副本/相似前缀文件.md']
  ].map(([title,path], i) => ({...note, title, path, id:'folder-fixture-'+i}));
  let activeId = 'test-workspace';
  let ui = JSON.parse(localStorage.getItem('test-ui-preferences') || 'null') || {fontSize:14,collapsedWorkspaces:[],entryOrder:{}};
  const workspaces = [{id:'test-workspace',name:'我的知识花园',path:'D:/Notes'},{id:'second-workspace',name:'第二工作区',path:'D:/Second'}];
  const secondNote = {...note,id:'second-note',title:'第二空间笔记',path:'独立目录/第二空间笔记.md',content:'# 第二空间笔记'};
  window.go = { main: { App: {
   DataDirectory: async () => 'D:/IsolatedTest/SmartNoteData',
   EmbeddingStatus: async()=>({state:'ready',model:'BAAI/bge-small-zh-v1.5',dimensions:512,message:'内置中文模型已就绪 · 全程离线',cached:0}),
   CheckEmbedding: async()=>({state:'ready',model:'BAAI/bge-small-zh-v1.5',dimensions:512,message:'内置中文模型已就绪 · 全程离线',cached:1}),
   SemanticSearch:async query=>({results:await window.go.main.App.Search(query),mode:'hybrid',message:'内置模型语义检索 + 关键词匹配；相似度不是概率'}),
   GetSemanticConfig: async()=>structuredClone(semanticConfig),
   SaveSemanticConfig: async(id,c)=>{semanticConfig={...structuredClone(c),revision:'semantic-'+Date.now()};return structuredClone(semanticConfig)},
   AutoArchive: async(workspaceId,noteId,hash)=>{
    const source=[note,...directoryFixtures,...created].find(n=>n.id===noteId);
    if(!semanticConfig.automatic || !source || (source.path.includes('/') && !/^Inbox\/[^/]+$/i.test(source.path)) || source.content.length<20 || !['自动语义测试','根目录自动整理测试'].includes(source.title))return null;
    return window.go.main.App.ApplyArchive({workspaceId,noteId,hash,directory:'技术/数据库',tags:[]})
   },
   SuggestSemanticArchive: async(workspaceId,noteId,hash)=>({workspaceId,noteId,hash,directory:'技术/数据库',tags:[],source:'semantic',ruleId:'db',reasons:['模型 BAAI/bge-small-zh-v1.5：最匹配「数据库」，余弦相似度 0.850']}),
   GetArchiveRules: async () => structuredClone(archiveConfig),
   SaveArchiveRules: async (workspaceId,config) => { if(workspaceId!==activeId || config.revision!==archiveConfig.revision) throw new Error('CONFLICT'); archiveConfig={...structuredClone(config),revision:'rules-'+Date.now()};return structuredClone(archiveConfig) },
   SuggestArchive: async (workspaceId,id,hash) => {
    const source=[note,...directoryFixtures,...created].find(n=>n.id===id);if(!source || source.hash!==hash || workspaceId!==activeId) throw new Error('CONFLICT');
    const rule=archiveConfig.rules.find(r=>r.enabled && r.keywords.some(key=>source.title.includes(key)));
    return {workspaceId,noteId:id,hash,directory:rule?.directory || source.path.split('/').slice(0,-1).join('/') || '.',tags:rule?.tags || [],reasons:[rule?'命中规则「'+rule.name+'」':'保持当前目录'],source:rule?'rule':'current',ruleId:rule?.id || ''}
   },
   ApplyArchive: async input => {
    const all=[note,...directoryFixtures,...created],source=all.find(n=>n.id===input.noteId);
    if(!source || source.hash!==input.hash || input.workspaceId!==activeId) throw new Error('CONFLICT');
    const path=(input.directory==='.'?'':input.directory+'/')+source.path.split('/').pop();if(all.some(n=>n.id!==source.id && n.path===path)) throw new Error('目标已存在');
    const before=structuredClone(source),operationId='archive-'+Date.now();
    const tags=[...new Set([...source.tags,...input.tags])];
    Object.assign(source,{path,tags,hash:operationId,content:source.content+'\n整理标签: '+tags.join(',')});
    const parts=input.directory.split('/');while(parts.length) {const dir=parts.join('/');if(dir!=='.' && !directories.includes(dir)) directories.push(dir);parts.pop()}
    archiveUndo.set(operationId,before);return {note:structuredClone(source),operationId,recoveryPath:'D:/IsolatedTest/archive',updatedLinks:1}
   },
   UndoArchive: async (workspaceId,operationId,hash) => {
    const before=archiveUndo.get(operationId),source=[note,...directoryFixtures,...created].find(n=>n.id===before?.id);
    if(!before || !source || source.hash!==hash || source.hash!==operationId || workspaceId!==activeId) throw new Error('笔记已修改，不能撤销');
    Object.assign(source,before);archiveUndo.delete(operationId);return {note:structuredClone(source),operationId:'',recoveryPath:'D:/IsolatedTest/archive',updatedLinks:1}
   },

   SetUIPreferences: async value => { ui = value; localStorage.setItem('test-ui-preferences',JSON.stringify(value)) },
   OpenWorkspace: async id => { activeId=id },
   ListWorkspaceFiles: async id => id==='second-workspace' ? {notes:[secondNote],directories:['独立目录']} : {notes:[note,...directoryFixtures,...created],directories},
   Snapshot: async () => ({ settings: { activeId, ui, theme: 'light', picGoUrl: '', workspaces }, directories, notes: activeId==='second-workspace' ? [secondNote] : [note,...directoryFixtures,...created], tasks: [{ id:'task1', noteId:note.id, noteTitle:note.title, title:'完善附件备份流程', line:19, done:false, candidate:false, project:'SmartNote',tags:['产品设计'],...details },{ id:'candidate1', noteId:note.id, noteTitle:note.title,title:'明天需要整理离线检索的测试资料。',line:30, done:false,candidate:!confirmed, dueAt:'',priority:'normal',project:'',tags:[] }], attachments: [] }),
   SaveNote: async input => {
    const all=[note,...directoryFixtures,...created]; const source=all.find(n=>n.path===input.path);
    if(!source || source.hash!==input.hash) throw new Error('CONFLICT');
    const title=input.content.match(/^---\r?\n[\s\S]*?^title:\s*(.+)$/m)?.[1]?.trim();
    const path=title && title!==source.title ? source.path.slice(0,source.path.lastIndexOf('/')+1)+title+'.md' : source.path;
    if(all.some(n=>n.id!==source.id && n.path===path)) throw new Error('目标已存在，移动不会覆盖文件');
    Object.assign(source,{content:input.content,path,title:title || source.title,hash:'saved-'+Date.now()}); return {...source}
   },
   CreateNote: async (title, dir) => { const n = { ...note, id: 'created-' + created.length, title, path: (dir === '.' ? '' : dir + '/') + title + '.md', content: '# ' + title + (title==='根目录自动整理测试'?'\n数据库查询与索引优化，记录问题排查及最终解决方案，需要整理到数据库分类。':''), hash: 'new-hash', tags: [] }; created.push(n); return n },
   Search: async () => [{note, snippet:'好的工具，让我们把注意力留给思考本身。',line:10,score:1}], Graph: async () => [],
   SetTask: async () => { confirmed = true }, SetPreferences: async () => {},
   UpdateTaskDetails: async input => { details = {dueAt:input.dueAt,priority:input.priority} },
   CreateDirectory: async path => { if(directories.includes(path)) throw new Error('目标已存在'); const parts=path.split('/'); while(parts.length) { const dir=parts.join('/'); if(!directories.includes(dir)) directories.push(dir); parts.pop() } },
   MoveDirectory: async (source,target) => {
    if(target===source || target.startsWith(source+'/') || directories.includes(target)) throw new Error('目标已存在或非法');
    const map=path=>path===source || path.startsWith(source+'/')?target+path.slice(source.length):path;
    for(const n of [note,...directoryFixtures,...created]) n.path=map(n.path);
    directories=directories.map(map); return {path:target,recoveryPath:'D:/Notes/.recovery',updatedLinks:1}
   },
   DeleteDirectory: async path => { for(const list of [directoryFixtures,created]) for(let i=list.length-1;i>=0;i--) if(list[i].path.startsWith(path+'/')) list.splice(i,1); directories=directories.filter(dir=>dir!==path && !dir.startsWith(path+'/')); return {path,recoveryPath:'D:/Notes/.smartnote-trash',updatedLinks:0} },
   DeleteNote: async (id,hash) => { for(const list of [directoryFixtures,created]) {const i=list.findIndex(n=>n.id===id); if(i>=0) {if(list[i].hash!==hash) throw new Error('CONFLICT'); list.splice(i,1); return}} },
   EntryLocation: async path => 'D:/Notes/'+path,
   RevealEntry: async () => {},
   MoveNote: async (id,hash,target) => {
    const all=[note,...directoryFixtures,...created]; const source=all.find(item=>item.id===id);
    if(all.some(item=>item.path===target)) throw new Error('目标已存在，移动不会覆盖文件');
    if(!source || source.hash!==hash) throw new Error('CONFLICT: 笔记已被外部修改');
    const moved={...source,path:target,hash:'moved-'+Date.now()};
    if(id===note.id) note=moved; else { const list=directoryFixtures.some(item=>item.id===id)?directoryFixtures:created; list[list.findIndex(item=>item.id===id)]=moved }
    return {note:moved,updatedLinks:2,recoveryPath:'D:/IsolatedTest/recovery'}
   },
  } } };
 });
 await page.reload(); await page.getByRole('heading', { name: '构建本地优先的知识库', exact: true }).first().waitFor();
 await page.locator('.workspace-root-name').first().click();
 const listTitles = () => page.locator('.note-list .note-card h3').allTextContents();
 if(JSON.stringify(await listTitles()) !== JSON.stringify(['根层文件'])) throw new Error('Root directory includes descendant files');
 await page.getByTitle('进入 层级测试',{exact:true}).click();
 if(JSON.stringify(await listTitles()) !== JSON.stringify(['父层文件'])) throw new Error('Parent directory includes child or sibling files');
 await page.getByTitle('进入 层级测试/子目录',{exact:true}).click();
 if(JSON.stringify(await listTitles()) !== JSON.stringify(['子层文件'])) throw new Error('Child directory includes parent files');
 await page.getByTitle('返回上级目录').click();
 if(JSON.stringify(await listTitles()) !== JSON.stringify(['父层文件'])) throw new Error('Return to parent failed');
 await page.locator('.tree-entry[title="空文件夹/子目录"]').click();
 await page.getByText('当前目录为空',{exact:true}).waitFor();
 if(await page.locator('.note-list .note-card, .note-list .directory-card').count()) throw new Error('Empty folder contains unrelated entries');
 await page.locator('.tree-entry[title="层级测试"]').click();
 if(JSON.stringify(await listTitles()) !== JSON.stringify(['父层文件'])) throw new Error('Sidebar directory selection is recursive');
 await page.locator('.main-nav button').filter({hasText:'全部笔记'}).click();
 await page.locator('.note-card').first().click(); await page.locator('.cm-content').waitFor();
 // Closing both panes must not prevent reopening the selected navigation file.
 await page.getByRole('button',{name:'隐藏编辑器',exact:true}).click();
 await page.getByRole('button',{name:'隐藏预览',exact:true}).click();
 await page.locator('.panels-empty').waitFor();
 await page.locator('.folder-tree .tree-file.selected').click();
 await page.locator('.cm-content').waitFor();
 await page.locator('.preview-pane').waitFor();
 await page.locator('.compact-editor-toolbar').getByTitle('预览',{exact:true}).click();
 await page.locator('.source-pane').waitFor({state:'detached'});
 await page.getByRole('button',{name:'隐藏预览',exact:true}).click();
 await page.locator('.folder-tree .tree-file.selected').click();
 await page.locator('.preview-pane').waitFor();
 await page.locator('.compact-editor-toolbar').getByTitle('分屏',{exact:true}).click();
 await page.locator('.cm-content').waitFor();
 await page.screenshot({path:'docs/preview-editor.png'});
 await page.locator('.cm-content').click(); await page.keyboard.press('Control+End'); await page.keyboard.type('\nSmoke test: autosave.'); await page.waitForTimeout(1400);
 if (!(await page.locator('.editor-status').innerText()).includes('已保存到本地')) throw new Error('Auto save did not finish');
 // Internal note drag inserts a relative local reference without moving the source.
 const originalPath=await page.locator('.folder-tree .tree-file.selected').getAttribute('title');
 await page.locator('.cm-content').click();await page.keyboard.press('Control+End');
 await page.locator('[data-workspace-id="test-workspace"] .tree-file[title="根层文件.md"]').dragTo(page.locator('.cm-line').last());
 await page.waitForFunction(()=>document.querySelector('.cm-content').textContent.includes('[根层文件](../%E6%A0%B9%E5%B1%82%E6%96%87%E4%BB%B6.md)'));
 await page.locator('.markdown-preview a').filter({hasText:'根层文件'}).click();
 await page.locator('.tree-file.selected[title="根层文件.md"]').waitFor();
 await page.getByRole('button',{name:'返回源文件',exact:true}).click();
 await page.waitForFunction(path=>document.querySelector('.tree-file.selected')?.getAttribute('title')===path,originalPath);
 await page.getByRole('button',{name:'前进',exact:true}).click();
 await page.locator('.tree-file.selected[title="根层文件.md"]').waitFor();
 const historyCdp=await page.context().newCDPSession(page);
 const historyPoint=await page.locator('.preview-pane').boundingBox();
 async function sideButton(button,buttons) {
  await historyCdp.send('Input.dispatchMouseEvent',{type:'mousePressed',x:historyPoint.x+40,y:historyPoint.y+80,button,buttons,clickCount:1});
  await historyCdp.send('Input.dispatchMouseEvent',{type:'mouseReleased',x:historyPoint.x+40,y:historyPoint.y+80,button,buttons:0,clickCount:1});
 }
 await sideButton('back',8);
 await page.waitForFunction(path=>document.querySelector('.tree-file.selected')?.getAttribute('title')===path,originalPath);
 await sideButton('forward',16);
 await page.locator('.tree-file.selected[title="根层文件.md"]').waitFor();
 await page.keyboard.press('Alt+ArrowLeft');
 await page.waitForFunction(path=>document.querySelector('.tree-file.selected')?.getAttribute('title')===path,originalPath);
 await page.locator('.tree-file[title="根层文件.md"]').click();
 if(!await page.getByRole('button',{name:'前进',exact:true}).isDisabled()) throw new Error('New navigation retained forward branch');
 await page.getByRole('button',{name:'返回源文件',exact:true}).click();
 await page.waitForFunction(path=>document.querySelector('.tree-file.selected')?.getAttribute('title')===path,originalPath);

 await page.locator('[data-workspace-id="test-workspace"] .tree-file').filter({hasText:originalPath.split('/').pop()}).click();
 await page.locator('.cm-content').click();await page.keyboard.press('Control+End');
 await page.locator('.main-nav button').filter({hasText:'全部笔记'}).click();
 await page.locator('.note-card').filter({has:page.getByRole('heading',{name:'根层文件',exact:true})}).dragTo(page.locator('.cm-line').last());
 await page.waitForFunction(()=>document.querySelector('.cm-content').textContent.split('[根层文件](').length===3);
 await page.locator('.cm-content').click();await page.keyboard.press('Control+z');
 await page.waitForFunction(()=>document.querySelector('.cm-content').textContent.split('[根层文件](').length===2);
 const beforeCrossWorkspace=await page.locator('.cm-content').innerText();
 await page.locator('.tree-file[title="独立目录/第二空间笔记.md"]').dragTo(page.locator('.cm-line').last());
 await page.getByText('请拖入当前工作区内的笔记以创建本地引用',{exact:true}).waitFor();
 if(await page.locator('.cm-content').innerText()!==beforeCrossWorkspace) throw new Error('Cross-workspace drag modified content');
 await page.locator('.main-nav button').filter({hasText:'待办事项'}).click(); await page.getByRole('button',{name:'候选任务'}).click(); await page.getByRole('button',{name:'确认为任务'}).click();
 await page.getByRole('button',{name:'待完成',exact:true}).click(); await page.getByText('明天需要整理离线检索的测试资料。',{exact:true}).waitFor();
 await page.getByTitle('设置日期和优先级').first().click(); await page.getByLabel('截止日期',{exact:true}).fill('2099-12-31'); await page.locator('.n-modal .n-base-selection').click(); await page.getByText('高',{exact:true}).last().click(); await page.getByRole('button',{name:'保存任务设置'}).click();
 await page.getByLabel('优先级筛选').selectOption('high'); await page.locator('.task-row').filter({hasText:'完善附件备份流程'}).waitFor(); if(await page.locator('.task-row').count()!==1)throw new Error('Priority filter failed');
 await page.getByLabel('日期筛选').selectOption('today'); await page.getByRole('heading',{name:'当前条件下没有任务'}).waitFor(); await page.getByLabel('日期筛选').selectOption('all');
 await page.screenshot({path:'docs/preview-tasks.png'});
 await page.locator('.main-nav button').filter({hasText:'搜索'}).click(); await page.locator('.search-query input').fill('知识'); await page.locator('.search-result').waitFor(); await page.locator('.search-result').click(); await page.locator('.editor-panel').waitFor();
 await page.getByTitle('移动或重命名笔记').click(); await page.getByPlaceholder('技术知识/新的笔记名称.md').fill('归档/测试移动.md'); await page.getByRole('button',{name:'移动并更新链接'}).click(); await page.locator('.folder-tree .tree-file.selected').filter({hasText:'测试移动.md'}).waitFor();
 await page.locator('.workspace-tree-row .tree-file').filter({hasText:'测试移动.md'}).waitFor();
 await page.locator('.topbar .new-note-button').click();
 await page.getByPlaceholder('例如：一个值得记录的想法').fill('手动路径笔记');
 await page.getByRole('combobox',{name:'保存目录',exact:true}).fill('工作/项目/设计');
 await page.getByRole('button',{name:'创建笔记',exact:true}).click();
 await page.locator('.folder-tree .tree-file.selected').filter({hasText:'手动路径笔记.md'}).waitFor();
 await page.getByTitle('折叠 工作/项目',{exact:true}).click();
 if(await page.locator('.workspace-tree-row .tree-file').filter({hasText:'手动路径笔记.md'}).count()) throw new Error('Collapsed folder still shows file');
 await page.getByTitle('展开 工作/项目',{exact:true}).click();
 await page.locator('.workspace-tree-row .tree-file').filter({hasText:'手动路径笔记.md'}).click();
 await page.locator('.topbar .new-note-button').click();
 await page.getByPlaceholder('例如：一个值得记录的想法').fill('空目录笔记');
 await page.getByRole('combobox',{name:'保存目录',exact:true}).fill('空文件夹');
 await page.getByRole('combobox',{name:'保存目录',exact:true}).press('Escape');
 await page.getByRole('button',{name:'展开保存目录',exact:true}).click();
 await page.getByRole('option',{name:'工作区根目录（.）',exact:true}).waitFor();
 await page.getByRole('option',{name:'空文件夹/子目录',exact:true}).click();
 await page.getByRole('button',{name:'创建笔记',exact:true}).click();
 await page.locator('.folder-tree .tree-file.selected').filter({hasText:'空目录笔记.md'}).waitFor();
 await page.locator('.workspace-tree-row .tree-file').filter({hasText:'测试移动.md'}).click();
 await page.locator('.folder-tree .tree-file.selected').filter({hasText:'测试移动.md'}).waitFor();
 if(await page.locator('.folder-tree .selected').count() !== 1 || !(await page.locator('.folder-tree .selected').getAttribute('class')).includes('tree-file')) throw new Error('Opening file also highlights parent folder');
 if((await page.locator('.folder-tree .tree-file.selected').innerText()).trim() !== '测试移动.md' || await page.locator('.note-list .note-path').count()) throw new Error('File UI still displays parent directory');
 await page.locator('.tree-entry[title="归档"]').click();
 if(await page.locator('.folder-tree .selected').count() !== 1 || await page.locator('.folder-tree .tree-file.selected').count()) throw new Error('Selecting directory retains file highlight');
 await page.getByTitle('折叠工作区 第二工作区',{exact:true}).click();
 if(await page.locator('[data-workspace-id="second-workspace"] .folder-tree').count()) throw new Error('Workspace collapse failed');
 await page.getByTitle('展开工作区 第二工作区',{exact:true}).click();
 await page.locator('.tree-file[title="独立目录/第二空间笔记.md"]').click();
 await page.locator('.folder-tree .tree-file.selected').filter({hasText:'第二空间笔记.md'}).waitFor();
 if(await page.locator('.note-list .note-card').count()!==1) throw new Error('Workspaces share note lists');
 await page.locator('[data-workspace-id="test-workspace"] .workspace-root-name').click();

 await page.getByTitle('上移 根层文件.md',{exact:true}).click();
 const savedOrder=await page.evaluate(()=>JSON.parse(localStorage.getItem('test-ui-preferences')).entryOrder['test-workspace']['']);
 if(savedOrder.indexOf('根层文件.md')>=savedOrder.length-1) throw new Error('Move up did not save order');
 const source=page.locator('[data-workspace-id="test-workspace"] .workspace-tree-row').filter({has:page.locator('.tree-entry[title="根层文件.md"]')});
 const target=page.locator('[data-workspace-id="test-workspace"] .workspace-tree-row').first();
 await page.setViewportSize({width:1440,height:1800}); await source.locator('.tree-grip').dragTo(target.locator('.tree-grip'));
 await page.waitForFunction(()=>JSON.parse(localStorage.getItem('test-ui-preferences')).entryOrder['test-workspace'][''][0]==='根层文件.md');
 if(!(await page.locator('.note-list > button').first().innerText()).includes('根层文件')) throw new Error('Right list does not follow custom order');
 const workspaceTree=page.locator('[data-workspace-id="test-workspace"]');
 const movedFile=()=>workspaceTree.locator('.workspace-tree-row').filter({has:page.locator('.tree-file[title="工作/项目/设计/手动路径笔记.md"]')});
 await page.getByTitle('折叠 空文件夹/子目录',{exact:true}).click();
 await movedFile().locator('.tree-grip').dragTo(workspaceTree.locator('.tree-entry[title="空文件夹/子目录"]'));
 await workspaceTree.locator('.tree-file[title="空文件夹/子目录/手动路径笔记.md"]').waitFor();
 if(await workspaceTree.locator('.tree-file[title="工作/项目/设计/手动路径笔记.md"]').count()) throw new Error('Cross-folder drop did not remove original path');
 await workspaceTree.locator('.tree-file[title="空文件夹/子目录/手动路径笔记.md"]').dragTo(workspaceTree.locator('.workspace-tree-row').filter({has:page.locator('.tree-entry[title="空文件夹"]')}).locator('.tree-toggle'));
 await workspaceTree.locator('.tree-file[title="空文件夹/手动路径笔记.md"]').waitFor();
 await workspaceTree.locator('.tree-file[title="空文件夹/手动路径笔记.md"]').dragTo(workspaceTree.locator('.workspace-root .tree-toggle'));

 await workspaceTree.locator('.tree-file[title="手动路径笔记.md"]').waitFor();
 await page.getByTitle('移动或重命名笔记').click();
 await page.getByPlaceholder('技术知识/新的笔记名称.md').fill('空文件夹/子目录/空目录笔记.md');
 await page.getByRole('button',{name:'移动并更新链接'}).click();
 await page.locator('.error-banner').filter({hasText:'目标已存在'}).waitFor();
 await page.getByRole('button',{name:'取消',exact:true}).click();
 await workspaceTree.locator('.tree-file[title="手动路径笔记.md"]').waitFor();
 await page.locator('.error-banner button').click();
 // Folder moves, context menu, scoped Delete/F2 and cut/paste.
 await workspaceTree.locator('.workspace-root-name').focus(); await page.keyboard.press('Control+Shift+N');
 await page.getByRole('textbox',{name:'文件夹名称',exact:true}).fill('拖动源//子/空/');
 await page.getByRole('button',{name:'创建文件夹',exact:true}).click();
 await workspaceTree.locator('.tree-entry[title="拖动源/子/空"]').waitFor();
 await workspaceTree.locator('.tree-entry[title="拖动源/子"]').click({button:'right'});
 await page.getByRole('menuitem',{name:/新建笔记/}).click();
 await page.getByPlaceholder('例如：一个值得记录的想法').fill('文件夹内笔记');
 if(await page.locator('.n-modal input').count()!==2) throw new Error('Directory picker is not a single editable field');
 await page.getByRole('button',{name:'创建笔记',exact:true}).click();
 await workspaceTree.locator('.tree-file[title="拖动源/子/文件夹内笔记.md"]').waitFor();
 await workspaceTree.locator('.tree-entry[title="拖动源"]').dragTo(workspaceTree.locator('.tree-entry[title="空文件夹"]'));
 await workspaceTree.locator('.tree-file[title="空文件夹/拖动源/子/文件夹内笔记.md"]').waitFor();
 await workspaceTree.locator('.tree-entry[title="空文件夹/拖动源"]').dragTo(workspaceTree.locator('.workspace-root .tree-toggle'));
 await workspaceTree.locator('.tree-entry[title="拖动源/子/空"]').waitFor();
 await workspaceTree.locator('.tree-entry[title="拖动源"]').dragTo(workspaceTree.locator('.tree-entry[title="拖动源/子"]'));
 await workspaceTree.locator('.tree-entry[title="拖动源/子/空"]').waitFor();
 await workspaceTree.locator('.tree-file[title="拖动源/子/文件夹内笔记.md"]').focus(); await page.keyboard.press('F2');
 await page.getByRole('textbox',{name:'新名称',exact:true}).fill('重命名笔记');
 await page.getByRole('button',{name:'确认重命名',exact:true}).click();
 await workspaceTree.locator('.tree-file[title="拖动源/子/重命名笔记.md"]').waitFor();
 await workspaceTree.locator('.tree-entry[title="拖动源"]').click({button:'right'});
 await page.getByRole('menuitem',{name:/重命名/}).click();
 await page.getByRole('textbox',{name:'新名称',exact:true}).fill('改名文件夹');
 await page.getByRole('button',{name:'确认重命名',exact:true}).click();
 await workspaceTree.locator('.tree-file[title="改名文件夹/子/重命名笔记.md"]').waitFor();
 await workspaceTree.locator('.tree-file[title="改名文件夹/子/重命名笔记.md"]').focus(); await page.keyboard.press('Control+x');
 await workspaceTree.locator('.tree-entry[title="空文件夹"]').focus(); await page.keyboard.press('Control+v');
 await workspaceTree.locator('.tree-file[title="空文件夹/重命名笔记.md"]').waitFor();
 await workspaceTree.locator('.tree-file[title="空文件夹/重命名笔记.md"]').click();
 await page.locator('.cm-content').click(); await page.keyboard.press('Delete');
 if(await page.getByText('移入回收目录？',{exact:true}).count()) throw new Error('Editor Delete opened file deletion');
 await workspaceTree.locator('.tree-file[title="空文件夹/重命名笔记.md"]').focus(); await page.keyboard.press('Delete');
 await page.getByRole('button',{name:'取消',exact:true}).click();
 await workspaceTree.locator('.tree-file[title="空文件夹/重命名笔记.md"]').focus(); await page.keyboard.press('Delete');
 await page.getByRole('button',{name:'移入回收目录',exact:true}).click();
 await workspaceTree.locator('.tree-file[title="空文件夹/重命名笔记.md"]').waitFor({state:'detached'});
 await workspaceTree.locator('.tree-entry[title="改名文件夹"]').click({button:'right'});
 await page.getByRole('menuitem',{name:/移入回收目录/}).click();
 await page.getByRole('button',{name:'移入回收目录',exact:true}).click();
 await workspaceTree.locator('.tree-entry[title="改名文件夹"]').waitFor({state:'detached'});
 // Editable archive rules -> explained preview -> apply -> safe undo.
 await page.setViewportSize({width:1440,height:1000});
 await page.getByRole('button',{name:'打开设置窗口',exact:true}).click();
 await page.getByRole('button',{name:'智能归档',exact:true}).click();
 await page.getByText('无需下载模型、填写地址或安装其他软件；笔记内容始终留在本机。',{exact:true}).waitFor();
 if(await page.getByLabel('Embedding 地址',{exact:true}).count())throw new Error('External model setup still shown');
 await page.getByLabel('分类名称',{exact:true}).fill('数据库资料');
 await page.getByLabel('分类说明',{exact:true}).fill('数据库、SQL和性能排查参考资料');
 await page.getByRole('button',{name:'新增分类',exact:true}).click();
 await page.getByLabel('分类名称',{exact:true}).last().fill('旅行');
 await page.getByLabel('分类说明',{exact:true}).last().fill('出行计划和景点攻略');
 await page.getByRole('button',{name:'删除分类 旅行',exact:true}).click();
 if(!await page.getByRole('checkbox',{name:'自动语义归档',exact:true}).isChecked())throw new Error('Automatic archive is not enabled by default');
 await page.getByRole('button',{name:'保存语义分类',exact:true}).click();
 await page.getByText('语义分类已保存',{exact:true}).waitFor();
 await page.getByRole('button',{name:'测试模型与分类',exact:true}).click();
 await page.locator('.semantic-settings').getByText('模型 BAAI/bge-small-zh-v1.5：最匹配「数据库」，余弦相似度 0.850',{exact:true}).waitFor();
 await page.screenshot({path:'docs/preview-v09-semantic.png'});

 await page.getByRole('button',{name:'添加规则',exact:true}).click();
 await page.getByLabel('规则名称',{exact:true}).fill('手动笔记归档');
 await page.getByLabel('关键词',{exact:true}).fill('手动路径');
 await page.getByLabel('推荐标签',{exact:true}).fill('数据库，排查');
 await page.locator('.archive-rules').getByRole('combobox',{name:'保存目录',exact:true}).fill('智能整理/归档');
 await page.getByRole('button',{name:'保存归档规则',exact:true}).click();
 await page.getByText('归档规则已保存',{exact:true}).waitFor();
 await page.getByLabel('规则名称',{exact:true}).fill('切页草稿');
 await page.getByRole('button',{name:'通用设置',exact:true}).click();
 await page.getByRole('button',{name:'智能归档',exact:true}).click();
 if(await page.getByLabel('规则名称',{exact:true}).inputValue()!=='切页草稿') throw new Error('Archive rule draft lost on settings tab switch');
 await page.getByLabel('规则名称',{exact:true}).fill('手动笔记归档');
 await page.screenshot({path:'docs/preview-v07-archive-rules.png'});

 await page.getByRole('button',{name:'关闭设置',exact:true}).click();
 await workspaceTree.locator('.tree-file[title="手动路径笔记.md"]').click();
 await page.getByRole('button',{name:'智能整理',exact:true}).click();
 await page.locator('.archive-reasons').getByText('模型 BAAI/bge-small-zh-v1.5：最匹配「数据库」，余弦相似度 0.850',{exact:true}).waitFor();
 await page.getByRole('checkbox',{name:'使用内置模型语义分类',exact:false}).uncheck();
 await page.getByText('命中规则「手动笔记归档」',{exact:true}).waitFor();
 await page.getByRole('checkbox',{name:'采用标签 排查',exact:true}).uncheck();
 await page.getByRole('button',{name:'隐藏智能整理',exact:true}).click();
 await page.getByRole('button',{name:'面板布局',exact:true}).click();
 await page.getByRole('menuitem',{name:'恢复智能整理',exact:true}).click();
 if(await page.getByRole('checkbox',{name:'采用标签 排查',exact:true}).isChecked()) throw new Error('Minimized archive lost tag selection');
 await page.waitForTimeout(350);
 await page.screenshot({path:'docs/preview-v07-archive-preview.png'});
 await page.getByRole('button',{name:'确认整理',exact:true}).click();
 await workspaceTree.locator('.tree-file.selected[title="智能整理/归档/手动路径笔记.md"]').waitFor();
 await page.getByRole('button',{name:'撤销上次整理',exact:true}).click();
 await workspaceTree.locator('.tree-file.selected[title="手动路径笔记.md"]').waitFor();

 // Inbox auto archive follows the moved note and retains the editor, with undo.
 await page.locator('.topbar .new-note-button').click();
 await page.getByPlaceholder('例如：一个值得记录的想法').fill('自动语义测试');
 await page.getByRole('combobox',{name:'保存目录',exact:true}).fill('Inbox');
 await page.getByRole('button',{name:'创建笔记',exact:true}).click();
 await workspaceTree.locator('.tree-file.selected[title="Inbox/自动语义测试.md"]').waitFor();
 await page.locator('.cm-content').click();await page.keyboard.press('Control+a');
 await page.keyboard.insertText('# 自动语义测试\n\n数据库慢查询优化与索引分析，需要记录排查过程以及解决方案。');
 await workspaceTree.locator('.tree-file.selected[title="技术/数据库/自动语义测试.md"]').waitFor({timeout:15000});
 if(!(await page.locator('.cm-content').innerText()).includes('数据库慢查询优化'))throw new Error('Auto archive hid editor or lost body');
 await page.getByRole('button',{name:'撤销上次整理',exact:true}).click();
 await workspaceTree.locator('.tree-file.selected[title="Inbox/自动语义测试.md"]').waitFor();

 // An already saved root note is organized on opening without editing or confirmation.
 await page.locator('.topbar .new-note-button').click();
 await page.getByPlaceholder('例如：一个值得记录的想法').fill('根目录自动整理测试');
 await page.getByRole('combobox',{name:'保存目录',exact:true}).fill('.');
 await page.getByRole('button',{name:'创建笔记',exact:true}).click();
 await workspaceTree.locator('.tree-file.selected[title="技术/数据库/根目录自动整理测试.md"]').waitFor({timeout:15000});
 if(await page.getByRole('button',{name:'确认整理',exact:true}).isVisible())throw new Error('Automatic archive requested confirmation');
 await page.getByRole('button',{name:'撤销上次整理',exact:true}).click();
 await workspaceTree.locator('.tree-file.selected[title="根目录自动整理测试.md"]').waitFor();
 await page.waitForTimeout(4500);
 if(!await workspaceTree.locator('.tree-file.selected[title="根目录自动整理测试.md"]').isVisible())throw new Error('Undo was immediately archived again');
 // Save response path changes update active editor, file tree and list immediately.
 await workspaceTree.locator('.tree-file[title="手动路径笔记.md"]').click();
 await page.locator('.cm-content').click(); await page.keyboard.press('Control+a');
 await page.keyboard.insertText('---\ntitle: 同步标题\n---\n\n保留编辑内容');
 await page.waitForFunction(()=>document.querySelector('.folder-tree .tree-file.selected')?.textContent.includes('同步标题.md'));
 await workspaceTree.locator('.tree-file[title="同步标题.md"]').waitFor();
 if(!(await page.locator('.cm-content').innerText()).includes('保留编辑内容')) throw new Error('Title rename lost editor text');
 await page.locator('.cm-content').click(); await page.keyboard.press('Control+a');
 await page.keyboard.insertText('---\ntitle: 手动路径笔记\n---\n\n保留编辑内容');
 await page.waitForFunction(()=>document.querySelector('.folder-tree .tree-file.selected')?.textContent.includes('手动路径笔记.md'));
 await workspaceTree.locator('.tree-file[title="手动路径笔记.md"]').waitFor();
 await page.locator('.main-nav button').filter({hasText:'知识拓扑'}).click(); await page.locator('.graph-canvas canvas').first().waitFor();
 await page.locator('.sidebar-bottom button').first().click(); await page.getByRole('button',{name:'通用设置',exact:true}).click(); await page.getByLabel('界面字体大小',{exact:true}).fill('18'); await page.waitForFunction(()=>document.documentElement.style.fontSize==='18px');
 if(await page.getByLabel('界面字体',{exact:true}).locator('option').count()<20) throw new Error('Missing common fonts');
 await page.getByLabel('界面字体',{exact:true}).selectOption('SimSun');
 await page.waitForFunction(()=>document.documentElement.style.getPropertyValue('--ui-font-family').includes('SimSun'));
 await page.getByLabel('编辑器字体',{exact:true}).selectOption('SimHei');
 await page.getByLabel('编辑器字体大小',{exact:true}).fill('24');
 await page.waitForFunction(()=>getComputedStyle(document.querySelector('.sidebar')).fontFamily.includes('SimSun'));
 await page.getByRole('button',{name:'关闭设置',exact:true}).click();
 await workspaceTree.locator('.tree-file[title="手动路径笔记.md"]').click();
 await page.waitForFunction(()=>getComputedStyle(document.querySelector('.cm-scroller')).fontFamily.includes('SimHei') && getComputedStyle(document.querySelector('.cm-content')).fontSize==='24px');
 // v0.5 floating settings, shared zoom and persistent panel geometry.
 await page.locator('.cm-content').dispatchEvent('wheel',{deltaY:-100,ctrlKey:true});
 await page.waitForFunction(()=>document.documentElement.style.getPropertyValue('--note-font-size')==='25px');
 await page.locator('.cm-content').dispatchEvent('wheel',{deltaY:100,ctrlKey:true});
 await page.waitForFunction(()=>document.documentElement.style.getPropertyValue('--note-font-size')==='24px');
 await page.getByRole('button',{name:'打开设置窗口',exact:true}).click();
 const float=page.locator('.floating-window');
 const before=await float.boundingBox();
 await page.getByRole('separator',{name:'调整设置窗口大小',exact:true}).focus();
 await page.keyboard.press('ArrowLeft');
 await page.waitForFunction(w=>document.querySelector('.floating-window').getBoundingClientRect().width<w,before.width);
 const header=await page.locator('.floating-window-header').boundingBox();
 await page.mouse.move(header.x+80,header.y+20);await page.mouse.down();await page.mouse.move(header.x+110,header.y+40);await page.mouse.up();
 if((await float.boundingBox()).x===before.x) throw new Error('Settings header does not drag');
 await page.getByRole('button',{name:'最小化设置',exact:true}).click();
 await float.waitFor({state:'hidden'});
 await page.getByRole('button',{name:'打开设置窗口',exact:true}).click();
 if(await page.getByLabel('编辑器字体大小',{exact:true}).inputValue()!=='24') throw new Error('Wheel and settings differ');
 await page.getByRole('button',{name:'关闭设置',exact:true}).click();
 const divider=page.getByRole('separator',{name:'调整笔记列表宽度',exact:true});
 const listWidth=await page.locator('.note-list-panel').evaluate(el=>el.getBoundingClientRect().width);
 await divider.focus(); await page.keyboard.press('ArrowRight');
 await page.waitForFunction(w=>document.querySelector('.note-list-panel').getBoundingClientRect().width>w,listWidth);
 await page.waitForFunction(()=>JSON.parse(localStorage.getItem('test-ui-preferences')).layout.listWidth>292);
 await page.getByRole('button',{name:'隐藏笔记列表',exact:true}).click();
 await page.locator('.note-list-panel').waitFor({state:'detached'});
 await page.getByRole('button',{name:'面板布局',exact:true}).click();
 await page.getByRole('menuitemcheckbox',{name:'笔记列表',exact:true}).click();
 await page.locator('.note-list-panel').waitFor();
 await page.getByRole('button',{name:'面板布局',exact:true}).click();
 await page.locator('.topbar .new-note-button').click();
 await page.getByPlaceholder('例如：一个值得记录的想法').fill('隐藏草稿');
 await page.getByRole('button',{name:'隐藏给新想法一个名字',exact:true}).click();
 await page.getByRole('button',{name:'面板布局',exact:true}).click();
 await page.getByRole('menuitem',{name:'恢复新建笔记',exact:true}).click();
 if(await page.getByPlaceholder('例如：一个值得记录的想法').inputValue()!=='隐藏草稿') throw new Error('Hidden modal lost draft');
 await page.waitForTimeout(400);
 const modalBox=await page.locator('.n-modal.n-card').boundingBox();
 await page.mouse.move(modalBox.x+modalBox.width-3,modalBox.y+modalBox.height-3);
 await page.mouse.down(); await page.mouse.move(modalBox.x+modalBox.width+60,modalBox.y+modalBox.height+40); await page.mouse.up();
 await page.waitForTimeout(400);
 if((await page.locator('.n-modal.n-card').boundingBox()).width<=modalBox.width) throw new Error('Modal resize failed');
 await page.getByRole('button',{name:'取消',exact:true}).click();
 const cdp=await page.context().newCDPSession(page); await cdp.send('DOM.enable'); await cdp.send('CSS.enable');
 const {root}=await cdp.send('DOM.getDocument');
 const {nodeId}=await cdp.send('DOM.querySelector',{nodeId:root.nodeId,selector:'.cm-line'});
 const {fonts}=await cdp.send('CSS.getPlatformFontsForNode',{nodeId});
 console.log('Rendered editor fonts:',fonts.map(font=>font.familyName).join(', '));
 if(!fonts.some(font=>/SimHei|黑体/i.test(font.familyName))) throw new Error('Selected editor font was not rendered');
 await page.locator('.sidebar-bottom button').first().click();
 // Editable shortcuts reject conflicts and honor the replacement in explorer scope.
 await page.getByRole('button',{name:'快捷键',exact:true}).click();
 await page.getByRole('button',{name:'修改快捷键 重命名文件 / 文件夹',exact:true}).click();
 await page.keyboard.press('Control+Alt+r');
 await page.getByRole('alert').filter({hasText:'冲突'}).waitFor();
 await page.keyboard.press('Control+Alt+m');
 await page.waitForFunction(()=>JSON.parse(localStorage.getItem('test-ui-preferences')).shortcuts.rename==='Ctrl+Alt+M');
 await page.getByRole('button',{name:'修改快捷键 新建文件夹',exact:true}).click();
 await page.keyboard.press('Control+Alt+n');
 await page.getByRole('button',{name:'关闭设置',exact:true}).click();
 await workspaceTree.locator('.tree-file[title="手动路径笔记.md"]').focus();
 await page.keyboard.press('F2');
 if(await page.getByRole('textbox',{name:'新名称',exact:true}).count()) throw new Error('Old rename shortcut still active');
 await page.keyboard.press('Control+Alt+m');
 await page.getByRole('textbox',{name:'新名称',exact:true}).fill('快捷键改名');
 await page.getByRole('button',{name:'确认重命名',exact:true}).click();
 await workspaceTree.locator('.tree-file[title="快捷键改名.md"]').focus(); await page.keyboard.press('Control+Alt+m');
 await page.getByRole('textbox',{name:'新名称',exact:true}).fill('手动路径笔记');
 await page.getByRole('button',{name:'确认重命名',exact:true}).click();
 await workspaceTree.locator('.workspace-root-name').focus(); await page.keyboard.press('Control+Alt+n');
 await page.getByRole('textbox',{name:'文件夹名称',exact:true}).fill('快捷键新文件夹');
 await page.getByRole('button',{name:'创建文件夹',exact:true}).click();
 await workspaceTree.locator('.tree-entry[title="快捷键新文件夹"]').waitFor();
 await page.keyboard.press('Control+Shift+p');
 await page.getByRole('textbox',{name:'搜索命令',exact:true}).fill('分屏'); await page.keyboard.press('Enter');
 await page.getByRole('textbox',{name:'搜索命令',exact:true}).waitFor({state:'detached'});
 await page.keyboard.press('Control+p');
 await page.getByRole('textbox',{name:'快速查找笔记',exact:true}).fill('手动路径笔记'); await page.keyboard.press('Enter');
 await page.locator('.folder-tree .tree-file.selected').filter({hasText:'手动路径笔记.md'}).waitFor();
 await page.locator('.cm-content').click(); await page.keyboard.press('Control+f');
 await page.locator('[aria-label="页面内查找内容"]').waitFor(); await page.keyboard.press('Escape');
 await page.locator('.sidebar-bottom button').first().click();
 await page.getByRole('button',{name:'WebDAV 同步',exact:true}).click(); await page.getByRole('heading',{name:'此能力尚未实现'}).waitFor();
 await page.getByRole('button',{name:'关闭设置',exact:true}).click();
 await page.getByRole('button',{name:'隐藏笔记大纲',exact:true}).click();
 await page.waitForFunction(()=>JSON.parse(localStorage.getItem('test-ui-preferences')).layout.hiddenPanels.includes('inspector'));

 await page.getByTitle('折叠工作区 第二工作区',{exact:true}).click();
 await page.waitForFunction(()=>JSON.parse(localStorage.getItem('test-ui-preferences')).collapsedWorkspaces.includes('second-workspace'));
 await page.reload(); await page.locator('.workspace-root-name').first().waitFor();
 if(await page.locator('[data-workspace-id="second-workspace"] .folder-tree').count()) throw new Error('Collapsed workspace not restored');
 if(await page.locator('.inspector').count()) throw new Error('Hidden panel did not survive reload');
 if(await page.evaluate(()=>JSON.parse(localStorage.getItem('test-ui-preferences')).layout.listWidth)<=292) throw new Error('Panel width not restored');
 await page.getByRole('button',{name:'面板布局',exact:true}).click();
 await page.getByRole('menuitemcheckbox',{name:'笔记大纲',exact:true}).click();
 await page.getByRole('button',{name:'面板布局',exact:true}).click();
 if(await page.evaluate(()=>document.documentElement.style.fontSize)!=='18px') throw new Error('Font size not restored');
 if(!(await page.evaluate(()=>document.documentElement.style.getPropertyValue('--ui-font-family'))).includes('SimSun')) throw new Error('Font family not restored');
 if(await page.evaluate(()=>document.documentElement.style.getPropertyValue('--note-font-size'))!=='24px') throw new Error('Editor size not restored');
 if(await page.evaluate(()=>JSON.parse(localStorage.getItem('test-ui-preferences')).shortcuts.rename)!=='Ctrl+Alt+M') throw new Error('Shortcuts not restored');
 const firstRootRow=page.locator('[data-workspace-id="test-workspace"] .workspace-tree-row').first();
 if(!(await firstRootRow.innerText()).includes('根层文件.md')) throw new Error('Custom order not restored');
 await page.evaluate(()=>window.dispatchEvent(new Event('blur')));
 await page.setViewportSize({width:1440,height:920}); await page.screenshot({path:'docs/preview-v03-sidebar.png'});
 await page.getByRole('button',{name:'打开设置窗口',exact:true}).click();
 await page.screenshot({path:'docs/preview-v05-floating-settings.png'});
 await browser.close();
 if(errors.length) throw new Error(errors.join('\n'));
 console.log('PASS: empty navigation, editor, auto save, candidate confirmation, task details/date/priority filters, search/open, move UI, direct directory children/root/parent navigation and empty folder isolation, nested directory input, empty folder selection, file tree collapse/open, multi-workspace collapse/switch, dropdown directory selection, drag/button sorting, cross-folder/root file moves, collision preservation, font family/size and preference restore, folder drag parent/root and self-drop prevention, context menus, scoped Delete/F2, cut/paste moves, custom shortcuts/conflict/persistence, command palette/quick open/find, graph, semantic category CRUD/model testing/automatic Inbox archive and undo, archive rule editing/preview/apply/undo and minimized draft restoration, capability status, tree/list note-reference drops, preview local-link navigation, back/forward buttons and real CDP mouse side buttons, Alt+Left and history branch reset, undo and cross-workspace rejection, title-driven save rename and immediate navigation refresh, Ctrl+wheel synchronized zoom, floating settings drag/resize/minimize, panel resizing and persistent visibility, modal resize and draft restoration; zero browser errors.');
})().catch(e => { console.error(e); process.exit(1) });


