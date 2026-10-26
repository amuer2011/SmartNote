const {chromium}=require('playwright');
(async()=>{const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});try{
const page=await browser.newPage({viewport:{width:1440,height:1000}}),errors=[];page.on('pageerror',e=>errors.push(String(e)));
await page.goto(process.env.SMARTNOTE_TEST_URL||'http://127.0.0.1:4175');
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
   GetBookmarks:async()=>JSON.parse(localStorage.getItem('test-bookmarks-'+activeId)||'[]'),
   AddBookmark:async input=>{const items=JSON.parse(localStorage.getItem('test-bookmarks-'+activeId)||'[]'),lines=note.content.split('\n'),b={...input,id:'bookmark-'+Date.now(),text:lines[input.line],before:lines[input.line-1]||'',after:lines[input.line+1]||''};items.push(b);localStorage.setItem('test-bookmarks-'+activeId,JSON.stringify(items));return b},
   DeleteBookmark:async(id,key)=>{const items=JSON.parse(localStorage.getItem('test-bookmarks-'+id)||'[]');localStorage.setItem('test-bookmarks-'+id,JSON.stringify(items.filter(item=>item.id!==key)))},
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
   Search: async () => [{note, snippet:'好的工具，让我们把注意力留给思考本身。',line:10,score:1}], Graph: async () => [{source:'test-note-1',target:'folder-fixture-0',type:'MARKDOWN_LINK'},{source:'folder-fixture-0',target:'folder-fixture-1',type:'MARKDOWN_LINK'}],
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



await page.reload();await page.getByRole('tab',{name:'导航',exact:true}).click();await page.getByRole('button',{name:'展开全部目录',exact:true}).click();
await page.locator('.tree-file').filter({hasText:'构建本地优先的知识库'}).first().click();
if(await page.locator('.source-pane').count())throw Error('Initial note is not preview');
await page.getByTitle('编辑',{exact:true}).click();
await page.locator('.tree-file').filter({hasText:'根层文件'}).first().click();
if(!await page.locator('.source-pane').count()||await page.locator('.preview-pane').count())throw Error('View not retained across notes');
await page.getByTitle('预览',{exact:true}).click();
await page.keyboard.press('Control+r');const bar=page.locator('.note-find-bar');await bar.waitFor();
await page.getByRole('textbox',{name:'页面内查找内容',exact:true}).fill('Markdown');
await bar.getByRole('button',{name:'正则表达式',exact:true}).click();
await page.getByRole('textbox',{name:'页面内查找内容',exact:true}).fill('(Mark)(down)');
await page.getByRole('textbox',{name:'页面内替换内容',exact:true}).fill('$2$1');
await bar.getByRole('button',{name:'全部替换',exact:true}).click();
await page.waitForFunction(()=>document.querySelector('.markdown-preview').textContent.includes('downMark'));
if(await page.locator('.source-pane').count())throw Error('Replace switched preview mode');
await page.getByRole('textbox',{name:'页面内查找内容',exact:true}).fill('[');await bar.getByRole('alert').waitFor();
await bar.getByRole('button',{name:'关闭页面内检索',exact:true}).click();
await page.getByRole('tab',{name:'搜索',exact:true}).click();
await page.locator('[aria-label="全局搜索模式"]').click();await page.getByText('正则查找与替换',{exact:true}).click();
await page.getByRole('textbox',{name:'全局查找内容',exact:true}).fill('Mark(down)');
await page.getByRole('button',{name:'展开替换',exact:true}).click();await page.getByRole('textbox',{name:'全局替换内容',exact:true}).fill('TEXT-$1');
await page.getByRole('button',{name:'预览全部替换',exact:true}).click();await page.locator('.replacement-preview').waitFor();
const planned=await page.locator('.replace-preview-item').count();if(planned<2)throw Error('Global replace did not include all notes');
await page.getByRole('button',{name:'应用以上替换',exact:true}).click();
await page.waitForFunction(()=>!document.querySelector('.replacement-preview'));
const snapshot=await page.evaluate(()=>window.go.main.App.Snapshot());
if(snapshot.notes.some(n=>n.content.includes('Markdown')))throw Error('Global regex replacement incomplete');
if(!snapshot.notes.some(n=>n.content.includes('TEXT-down')))throw Error('Global capture replacement failed');
await page.getByRole('textbox',{name:'全局查找内容',exact:true}).fill('TEXT-down');await page.getByRole('textbox',{name:'全局替换内容',exact:true}).fill('PARTIAL');
await page.getByRole('button',{name:'预览全部替换',exact:true}).click();
await page.evaluate(()=>{let calls=0;const save=window.go.main.App.SaveNote;window.go.main.App.SaveNote=async input=>{if(++calls===2)throw Error('CONFLICT simulated external edit');return save(input)}});
await page.getByRole('button',{name:'应用以上替换',exact:true}).click();await page.locator('.workspace-text-search [role="alert"]').filter({hasText:'CONFLICT'}).waitFor();
await page.locator('.workspace-text-search [role="status"]').filter({hasText:'已替换 1 篇笔记'}).waitFor();
await page.keyboard.press('Control+Shift+r');await page.getByRole('textbox',{name:'全局查找内容',exact:true}).waitFor();
await page.getByRole('textbox',{name:'全局查找内容',exact:true}).fill('[');await page.locator('.workspace-text-search [role="alert"]').waitFor();
await page.getByRole('tab',{name:'知识拓扑',exact:true}).click();await page.getByRole('button',{name:'全局',exact:true}).click();
const graph=page.locator('.graph-main-overlay .graph-canvas');await graph.waitFor();await page.waitForTimeout(200);
const layout=await graph.evaluate(el=>{const cy=el._cyreg.cy;return {nodes:cy.nodes().map(n=>({id:n.id(),position:{...n.position()},box:n.boundingBox({includeLabels:true})})),zoom:cy.zoom(),pan:{...cy.pan()}}});
if(layout.nodes.length<3)throw Error('Missing global graph nodes');
for(let i=0;i<layout.nodes.length;i++)for(let j=i+1;j<layout.nodes.length;j++){const a=layout.nodes[i].box,b=layout.nodes[j].box;if(a.x1<b.x2&&a.x2>b.x1&&a.y1<b.y2&&a.y2>b.y1)throw Error('Overlapping graph labels')}
await graph.evaluate(el=>{const cy=el._cyreg.cy;cy.zoom(.9);cy.pan({x:180,y:120})});
await page.getByRole('button',{name:'关闭知识拓扑',exact:true}).click();await page.setViewportSize({width:1350,height:900});await page.getByRole('tab',{name:'知识拓扑',exact:true}).click();await page.waitForTimeout(150);
const reopened=await graph.evaluate(el=>({zoom:el._cyreg.cy.zoom(),pan:el._cyreg.cy.pan()}));if(reopened.zoom!==.9||reopened.pan.x!==180||reopened.pan.y!==120)throw Error('Graph viewport reset');
await page.getByRole('button',{name:'重新布局拓扑',exact:true}).click();await page.waitForTimeout(100);await page.screenshot({path:'../docs/preview-natural-graph.png'});
if(errors.length)throw Error(errors.join('\n'));console.log('PASS: default preview, retained view, Ctrl+R, regex note replacement without mode switch, invalid regex, global preview/capture replacement across all notes');
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
