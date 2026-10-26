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



await page.addInitScript(()=>{
 const api=window.go.main.App,original=api.Snapshot;
 const key=d=>`${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`;
 const now=key(new Date());let done=false;window.calendarCalls={holiday:{},lunar:{}};
 api.Snapshot=async()=>{const data=await original();data.tasks=[{id:'note-intent',noteId:data.notes[0].id,noteTitle:'下周五提醒我VPN延期',title:'下周五提醒我VPN延期',line:0,inferred:true,candidate:false,done,dueAt:now,priority:'normal',project:'SmartNote',tags:[]}];return data};
 api.SetTask=async(id,value)=>{done=value};
 api.GetHolidayYear=async year=>{window.calendarCalls.holiday[year]=(window.calendarCalls.holiday[year]||0)+1;return {year,holiday:{},type:{[year+'-10-01']:{type:2,name:'国庆节',week:4},[year+'-10-10']:{type:3,name:'国庆节后补班',week:6}}}};
 api.GetLunarYear=async year=>{window.calendarCalls.lunar[year]=(window.calendarCalls.lunar[year]||0)+1;const days={};for(let d=new Date(year,0,1,12);d.getFullYear()===year;d.setDate(d.getDate()+1))days[key(d)]={month:'九月',day:'初六',term:key(d)===year+'-10-23'?'霜降':''};return {year,version:1,days}};
 api.GetReminderConfig=async()=>({revision:'r',enabled:false,time:'09:00',leadDays:0});api.DueReminders=async()=>[];
});
await page.reload();await page.getByRole('tab',{name:'待办事项',exact:true}).click();
await page.locator('.task-main-overlay:visible').waitFor();
if(await page.locator('.schedule-group .task-row').count()!==1)throw Error('Missing inferred note task');
await page.getByRole('button',{name:'当日待办',exact:true}).click();
await page.locator('.day-strip').waitFor();if(await page.locator('.schedule-group .task-row').count()!==1)throw Error('Day filter failed');
await page.screenshot({path:'docs/preview-tasks-day.png'});
await page.getByRole('button',{name:'完成任务',exact:true}).click();await page.locator('.task-checkbox.checked').waitFor();
await page.getByRole('button',{name:'最近待办',exact:true}).click();await page.getByRole('heading',{name:/已完成\s*1/}).waitFor();
await page.screenshot({path:'docs/preview-tasks-recent.png'});
await page.getByRole('button',{name:'日程概览',exact:true}).click();
await page.getByLabel('日历月份',{exact:true}).fill('2026-10');await page.getByLabel('日历月份',{exact:true}).press('Enter');await page.locator('.calendar-cell .work').waitFor();
if(await page.locator('.calendar-cell').count()!==42)throw Error('Calendar not six weeks');
if(await page.getByTitle('国庆节后补班',{exact:true}).innerText()!=='班')throw Error('Adjusted workday not marked');
await page.locator('.calendar-date[aria-label="选择 2026-10-23"]').filter({hasText:'霜降'}).waitFor();
await page.screenshot({path:'docs/preview-tasks-calendar.png'});
const counts=await page.evaluate(()=>structuredClone(window.calendarCalls));
await page.getByRole('button',{name:'下个月',exact:true}).click();await page.waitForTimeout(100);await page.getByRole('button',{name:'上个月',exact:true}).click();
const next=await page.evaluate(()=>window.calendarCalls);if(JSON.stringify(counts)!==JSON.stringify(next))throw Error('Repeated year loads');
await page.locator('.calendar-date[aria-label="选择 2026-10-10"]').click();
await page.getByRole('button',{name:'最近待办',exact:true}).click();await page.locator('.task-title').click();
await page.locator('.task-main-overlay').waitFor({state:'hidden'});await page.locator('.preview-pane').waitFor();
if(errors.length)throw Error(errors.join('\n'));
console.log('PASS note-level tasks, daily/recent completion, main calendar, holiday/workday/lunar/solar-term markers, one frontend year load, source navigation');
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
