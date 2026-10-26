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
 const b=window.go.main.App,snapshot=b.Snapshot,save=b.SaveNote;
 window.savedInputs=[];window.downloads=[];
 const attachments=[{id:'json-demo',name:'配置.json',mime:'application/json',size:50},{id:'img-demo',name:'图像.png',mime:'image/png',size:100},{id:'bin-demo',name:'归档.zip',mime:'application/zip',size:200}];
 b.Snapshot=async()=>{const data=await snapshot();data.attachments=structuredClone(attachments);const n=data.notes.find(n=>n.id==='test-note-1');if(n&&!n.content.includes('json-demo'))n.content+='\n[配置.json](smartnote-asset://json-demo)\n![图片附件](smartnote-asset://img-demo)\n';return data};
 b.SaveNote=async input=>{window.savedInputs.push(input);return save(input)};
 b.PreviewAttachment=async(id,key)=>key==='json-demo'?{kind:'text',content:'{"message":"你好 <script>alert(1)</script>"}',message:''}:{kind:'file',content:'',message:'此类型附件暂不支持内容预览，可下载后查看。'};
 b.AttachmentData=async()=> 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jN1cAAAAASUVORK5CYII=';
 b.GetGraphAnnotations=async()=>({revision:'',nodes:{},relations:{}});
 b.AttachmentUsage=async()=>attachments.map(a=>({id:a.id,notes:a.id==='bin-demo'?(window.binUsage||0):1,trash:0}));
 window.attachmentDeletes=[];
 b.DeleteAttachments=async input=>{window.attachmentDeletes.push(input);if(window.failDelete){window.failDelete=false;throw Error('模拟删除失败，请重试')}const usage=await b.AttachmentUsage(),deleted=[],skipped={};for(const id of input.ids){const item=usage.find(a=>a.id===id);if(!item){skipped[id]='不存在';continue}if(!input.force&&(item.notes||item.trash)){skipped[id]='仍被引用';continue}deleted.push(id)}for(const id of deleted){const index=attachments.findIndex(a=>a.id===id);if(index>=0)attachments.splice(index,1)}return {deleted,skipped,warnings:[]}};
 b.SaveAttachment=async(id,name)=>window.downloads.push({id,name});
 const snapWithAttachments=b.Snapshot;
 b.Snapshot=async()=>{const data=await snapWithAttachments();data.settings.attachments=JSON.parse(localStorage.getItem('test-attachment-prefs')||'{"picGoEnabled":true,"keepOriginal":true,"deduplicate":true}');data.settings.picGoUrl=localStorage.getItem('test-picgo-url')||'';data.settings.theme=localStorage.getItem('test-attachments-theme')||'light';return data};
 b.SetPreferences=async theme=>localStorage.setItem('test-attachments-theme',theme);
 b.SetAttachmentPreferences=async(url,prefs)=>{localStorage.setItem('test-picgo-url',url);localStorage.setItem('test-attachment-prefs',JSON.stringify(prefs))};
 b.ImportAttachment=async(name,base64,paste)=>{const prefs=JSON.parse(localStorage.getItem('test-attachment-prefs')||'{"picGoEnabled":true,"keepOriginal":true,"deduplicate":true}');window.lastImport={name,paste};return {id:'pasted-image',name,mime:'image/png',size:80,sha256:'mock',remoteUrl:prefs.picGoEnabled?'https://cdn.example.test/paste.png':'',remoteOnly:prefs.picGoEnabled&&!prefs.keepOriginal}};

});
await page.reload();await page.getByTitle('展开全部目录',{exact:true}).waitFor();if(await page.locator('.tree-file').filter({hasText:'构建本地优先的知识库'}).count())throw Error('Nested notes were expanded at startup');await page.getByTitle('展开全部目录',{exact:true}).click();await page.locator('.tree-file').filter({hasText:'构建本地优先的知识库'}).first().click();await page.getByTitle('分屏',{exact:true}).click();
const body=page.locator('.cm-content');await body.waitFor();
if((await body.innerText()).includes('id: test-note-1'))throw Error('Metadata is not hidden');
if((await body.innerText()).includes('# 构建本地优先的知识库'))throw Error('Generated title is not hidden');
if(!/^修改于 \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(await page.locator('.note-edited-at').innerText()))throw Error('Timestamp format mismatch');
await page.getByRole('button',{name:'显示元数据面板',exact:true}).click();if(!(await body.innerText()).includes('id: test-note-1'))throw Error('Show metadata failed');
await page.getByRole('button',{name:'隐藏元数据面板',exact:true}).click();
await body.click();await page.keyboard.press('Control+End');await page.keyboard.type('toolbar-target');for(let i=0;i<'toolbar-target'.length;i++)await page.keyboard.press('Shift+ArrowLeft');await page.keyboard.press('Control+b');if(!await page.locator('.preview-pane').isVisible()||!await body.isVisible())throw Error('Bold shortcut changed split mode');
await page.waitForTimeout(1100);const saved=await page.evaluate(()=>window.savedInputs.at(-1)?.content||'');
if(!saved.includes('**toolbar-target**')||!saved.startsWith('---\nid: test-note-1')||!saved.includes('# 构建本地优先的知识库'))throw Error('Formatting lost body or metadata');
await page.keyboard.press('ArrowRight');await page.keyboard.press('End');await page.keyboard.press('Enter');await page.keyboard.insertText('斜体效果');await page.waitForTimeout(150);await page.keyboard.press('End');await page.keyboard.press('Shift+Home');await page.keyboard.press('Control+i');
if(!await page.locator('.preview-pane').isVisible())throw Error('Italic shortcut changed split mode');
const italic=page.locator('.markdown-preview em').filter({hasText:'斜体效果'});await italic.waitFor();if(await italic.evaluate(el=>getComputedStyle(el).fontStyle)!=='italic'||!((await italic.evaluate(el=>getComputedStyle(el).fontSynthesis)).includes('style')))throw Error('Italic synthesis missing');
await page.locator('.preview-pane').getByRole('link',{name:'配置.json',exact:true}).click();await page.getByText('附件预览',{exact:true}).waitFor();
const modal=page.locator('.n-modal').filter({has:page.getByText('附件预览',{exact:true})});
await modal.locator('.attachment-text').waitFor();if(!(await modal.locator('.attachment-text').innerText()).includes('<script>'))throw Error('Attachment text altered');
if(await modal.locator('script').count())throw Error('Attachment executed as HTML');
await modal.getByRole('button',{name:'下载当前附件',exact:true}).click();await page.waitForFunction(()=>window.downloads.length===1);
await modal.locator('.n-base-close').click();
await page.getByRole('tab',{name:'附件库',exact:true}).click();if(await page.locator('.attachment-symbol').count())throw Error('Attachment cards were not removed');
await page.locator('.attachment-open').filter({hasText:'配置.json'}).click();await page.getByRole('region',{name:'主区域附件预览'}).locator('.attachment-text').waitFor();
await page.screenshot({path:'docs/preview-attachment-main-text.png'});
await page.locator('.attachment-open').filter({hasText:'归档.zip'}).click();await page.getByRole('region',{name:'主区域附件预览'}).locator('.attachment-file').waitFor();
await page.locator('.attachment-open').filter({hasText:'图像.png'}).click();await page.locator('.image-viewer').waitFor();await page.getByRole('button',{name:'下载图片附件',exact:true}).click();await page.waitForFunction(()=>window.downloads.some(a=>a.id==='img-demo'));
await page.locator('.image-stage img').click();if(!await page.locator('.image-viewer').isVisible())throw Error('Clicking image closed viewer');
const stage=page.locator('.image-stage'),box=await stage.boundingBox();await page.mouse.move(box.x+20,box.y+20);await page.mouse.down();await page.mouse.move(box.x+60,box.y+40);await page.mouse.up();if(!await page.locator('.image-viewer').isVisible())throw Error('Dragging blank space closed viewer');
await stage.click({position:{x:20,y:20}});if(await page.locator('.image-viewer').count())throw Error('Blank click did not close viewer');
await page.locator('.attachment-open').filter({hasText:'图像.png'}).click();await page.locator('.image-viewer').waitFor();await page.mouse.click(8,8);if(await page.locator('.image-viewer').count())throw Error('Library outer backdrop click did not close');if(await page.getByRole('tab',{name:'附件库',exact:true}).getAttribute('aria-selected')!=='true')throw Error('Backdrop click activated underlying tab');
// Use real-sized fixtures to verify pixels on screen, including CSS transform scaling.
async function openSizedImage(width,height){
 await page.evaluate(({width,height})=>{const canvas=document.createElement('canvas');canvas.width=width;canvas.height=height;const ctx=canvas.getContext('2d');ctx.fillStyle='#ece9f7';ctx.fillRect(0,0,width,height);ctx.fillStyle='#7770c4';ctx.font='24px sans-serif';ctx.fillText(`${width} × ${height}`,20,40);const src=canvas.toDataURL();window.go.main.App.AttachmentData=async()=>src},{width,height});
 await page.locator('.attachment-open').filter({hasText:'图像.png'}).click();await page.waitForFunction(()=>{const img=document.querySelector('.image-stage img');return img?.complete&&img.naturalWidth>1&&!img.classList.contains('image-pending')});await page.waitForTimeout(100);
}
async function imageBounds(){return page.locator('.image-stage img').boundingBox()}
const closeImage=()=>page.getByRole('button',{name:'关闭图片查看器',exact:true}).click();
await openSizedImage(320,180);let dimensions=await imageBounds(),panel=await page.locator('.image-viewer').boundingBox();
if(Math.abs(dimensions.width-320)>1||Math.abs(dimensions.height-180)>1||panel.width>470||panel.height>280)throw Error('Small image enlarged or window oversized: '+JSON.stringify({dimensions,panel}));
await page.screenshot({path:'docs/preview-image-natural-v0.12.4.png'});
// Move the window by its title bar; dragging the image continues to pan only its contents.
const centered=await page.locator('.image-viewer').boundingBox();
await page.mouse.move(centered.x+15,centered.y+24);await page.mouse.down();await page.mouse.move(centered.x+115,centered.y+84,{steps:10});await page.mouse.up();
const movedWindow=await page.locator('.image-viewer').boundingBox();
if(Math.abs(movedWindow.x-centered.x-100)>1||Math.abs(movedWindow.y-centered.y-60)>1)throw Error('Title drag did not move viewer');
const beforeImageDrag=await stage.boundingBox();
await page.mouse.move(beforeImageDrag.x+beforeImageDrag.width/2,beforeImageDrag.y+beforeImageDrag.height/2);await page.mouse.down();await page.mouse.move(beforeImageDrag.x+beforeImageDrag.width/2+25,beforeImageDrag.y+beforeImageDrag.height/2+15,{steps:4});await page.mouse.up();
let windowAfterImageDrag=await page.locator('.image-viewer').boundingBox();
if(Math.abs(windowAfterImageDrag.x-movedWindow.x)>1||Math.abs(windowAfterImageDrag.y-movedWindow.y)>1)throw Error('Image pan moved viewer');
await page.getByRole('button',{name:'适应窗口',exact:true}).click();
if(Math.abs((await page.locator('.image-viewer').boundingBox()).x-movedWindow.x)>1)throw Error('Toolbar button moved viewer');
await page.screenshot({path:'docs/preview-image-drag-v0.12.4.png'});
const ordinary=await page.locator('.image-viewer').boundingBox();
await page.getByRole('button',{name:'全屏显示图片',exact:true}).click();
await page.waitForFunction(()=>document.querySelector('.image-viewer').getBoundingClientRect().width===innerWidth);
let full=await page.locator('.image-viewer').boundingBox();if(full.x!==0||full.y!==0||full.height!==1000)throw Error('Fullscreen did not fill viewport');
if(Math.abs((await imageBounds()).width-320)>1)throw Error('Fullscreen enlarged small image');
await page.screenshot({path:'docs/preview-image-fullscreen-v0.12.4.png'});
await page.keyboard.press('Escape');await page.getByRole('button',{name:'全屏显示图片',exact:true}).waitFor();
let restored=await page.locator('.image-viewer').boundingBox();if(Math.abs(restored.x-ordinary.x)>1||Math.abs(restored.y-ordinary.y)>1||Math.abs(restored.width-ordinary.width)>1||Math.abs(restored.height-ordinary.height)>1)throw Error('Fullscreen lost original size');
await page.getByRole('button',{name:'全屏显示图片',exact:true}).click();await page.getByRole('button',{name:'恢复图片窗口',exact:true}).click();

await page.mouse.move(restored.x+15,restored.y+24);await page.mouse.down();await page.mouse.move(2500,1800,{steps:12});await page.mouse.up();
let clampedWindow=await page.locator('.image-viewer').boundingBox();if(clampedWindow.x+clampedWindow.width>1440||clampedWindow.y+clampedWindow.height>1000)throw Error('Window drag escaped viewport');
await page.setViewportSize({width:900,height:650});await page.waitForTimeout(100);clampedWindow=await page.locator('.image-viewer').boundingBox();if(clampedWindow.x<0||clampedWindow.y<0||clampedWindow.x+clampedWindow.width>900||clampedWindow.y+clampedWindow.height>650)throw Error('Moved window escaped smaller viewport');await page.setViewportSize({width:1440,height:1000});
await page.getByRole('button',{name:'放大',exact:true}).click();if((await imageBounds()).width<=320)throw Error('Image zoom did not enlarge');await page.getByRole('button',{name:'原始尺寸',exact:true}).click();if(Math.abs((await imageBounds()).width-320)>1)throw Error('Original size did not restore pixels');
const smallStage=await stage.boundingBox();await page.mouse.move(smallStage.x+smallStage.width/2,smallStage.y+smallStage.height/2);await page.mouse.wheel(0,-200);await page.waitForTimeout(100);if((await imageBounds()).width<=320)throw Error('Wheel zoom did not enlarge');await page.getByRole('button',{name:'适应窗口',exact:true}).click();
await page.getByRole('button',{name:'旋转',exact:true}).click();dimensions=await imageBounds();const rotatedStage=await stage.boundingBox();if(dimensions.width>rotatedStage.width||dimensions.height>rotatedStage.height)throw Error('Rotated image did not fit');
await page.getByRole('button',{name:'隐藏图片查看器',exact:true}).click();if(await page.locator('.image-viewer').count())throw Error('Minimize failed');await page.getByRole('button',{name:'面板布局',exact:true}).click();await page.getByRole('menuitem',{name:'恢复图片查看器',exact:true}).click();await page.locator('.image-stage img:not(.image-pending)').waitFor();await closeImage();
await openSizedImage(2400,1600);dimensions=await imageBounds();panel=await page.locator('.image-viewer').boundingBox();const largeStage=await stage.boundingBox();
if(dimensions.width>largeStage.width||dimensions.height>largeStage.height||dimensions.width>=2400||panel.width>1416||panel.height>976)throw Error('Large image did not fit screen');
await page.getByRole('button',{name:'原始尺寸',exact:true}).click();if(Math.abs((await imageBounds()).width-2400)>1)throw Error('Large image original size failed');await page.getByRole('button',{name:'适应窗口',exact:true}).click();
await page.screenshot({path:'docs/preview-image-fit-v0.12.4.png'});
// Exercise the browser resize handle, then check fitted content and a smaller viewport.
panel=await page.locator('.image-viewer').boundingBox();await page.mouse.move(panel.x+panel.width-8,panel.y+panel.height-8);await page.mouse.down();await page.mouse.move(panel.x+panel.width-183,panel.y+panel.height-123,{steps:8});await page.mouse.up();await page.waitForTimeout(150);const resized=await page.locator('.image-viewer').boundingBox();if(resized.width>=panel.width-30||resized.height>=panel.height-30)throw Error('Manual viewer resize failed: '+JSON.stringify({panel,resized}));
dimensions=await imageBounds();const resizedStage=await stage.boundingBox();if(dimensions.width>resizedStage.width||dimensions.height>resizedStage.height)throw Error('Image not fitted after resize');
await page.setViewportSize({width:800,height:650});await page.waitForTimeout(150);panel=await page.locator('.image-viewer').boundingBox();if(panel.x<0||panel.y<0||panel.x+panel.width>800||panel.y+panel.height>650)throw Error('Viewer escaped smaller viewport');await closeImage();
await openSizedImage(600,1800);dimensions=await imageBounds();panel=await page.locator('.image-viewer').boundingBox();if(dimensions.height>600||dimensions.width>=600||panel.width>776||panel.height>626)throw Error('Portrait image not fitted');await closeImage();await page.setViewportSize({width:1440,height:1000});
await page.evaluate(()=>{window.go.main.App.AttachmentData=async()=> 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jN1cAAAAASUVORK5CYII='});
await page.getByRole('tab',{name:'导航',exact:true}).click();await page.getByTitle('分屏',{exact:true}).click();await page.locator('.preview-pane img').click();await page.locator('.image-viewer').waitFor();await page.mouse.click(8,8);if(await page.locator('.image-viewer').count())throw Error('Note image outer backdrop did not close');
await page.screenshot({path:'docs/preview-editor-clean-toolbar.png'});
await body.click();await page.keyboard.press('Control+a');await page.keyboard.type('whole-body-replacement');await page.waitForTimeout(1100);
const replaced=await page.evaluate(()=>window.savedInputs.at(-1)?.content||'');if(!replaced.includes('whole-body-replacement')||!replaced.includes('id: test-note-1')||!replaced.includes('# 构建本地优先的知识库'))throw Error('Select all replaced protected metadata');
await page.keyboard.press('Control+z');await page.waitForTimeout(1100);if(!(await body.innerText()).includes('toolbar-target'))throw Error('Undo did not restore body');
await page.getByRole('button',{name:'打开设置窗口',exact:true}).click();const settings=page.getByRole('dialog',{name:'设置',exact:true});await settings.getByRole('button',{name:'附件与 PicGo',exact:true}).click();
for(const name of ['PicGo 上传','本地原图留存','附件去重']){const control=settings.getByRole('switch',{name,exact:true});if(await control.getAttribute('aria-checked')!=='true')throw Error('Attachment default disabled: '+name);await control.click()}
if(!await settings.getByLabel('PicGo HTTP Server 地址',{exact:true}).isDisabled())throw Error('Disabled PicGo left URL editable');await settings.getByRole('button',{name:'保存配置',exact:true}).click();await page.waitForFunction(()=>JSON.parse(localStorage.getItem('test-attachment-prefs')).deduplicate===false);await settings.getByRole('button',{name:'关闭设置',exact:true}).click();await page.reload();await page.getByRole('button',{name:'打开设置窗口',exact:true}).click();await settings.getByRole('button',{name:'附件与 PicGo',exact:true}).click();for(const name of ['PicGo 上传','本地原图留存','附件去重']){if(await settings.getByRole('switch',{name,exact:true}).getAttribute('aria-checked')!=='false')throw Error('Attachment settings lost on reload: '+name)}
await settings.getByRole('switch',{name:'PicGo 上传',exact:true}).click();await settings.getByRole('button',{name:'保存配置',exact:true}).click();await page.waitForFunction(()=>JSON.parse(localStorage.getItem('test-attachment-prefs')).picGoEnabled);await settings.locator('.settings-detail').evaluate(el=>el.scrollTop=0);await page.screenshot({path:'docs/preview-picgo-settings-v0.12.1.png'});await settings.getByRole('button',{name:'关闭设置',exact:true}).click();await page.getByTitle('切换主题',{exact:true}).click();await page.waitForFunction(()=>document.documentElement.dataset.theme==='dark');await page.getByRole('button',{name:'打开设置窗口',exact:true}).click();await settings.locator('.settings-detail').evaluate(el=>el.scrollTop=0);if((await settings.getByRole('switch',{name:'PicGo 上传',exact:true}).evaluate(el=>getComputedStyle(el).getPropertyValue('--n-rail-color-active'))).trim()!=='#7770c4')throw Error('Dark switch did not use app accent');await page.screenshot({path:'docs/preview-picgo-settings-dark-v0.12.1.png'});await settings.getByRole('button',{name:'关闭设置',exact:true}).click();await page.getByTitle('切换主题',{exact:true}).click();
await page.getByRole('button',{name:'展开全部目录',exact:true}).click();await page.locator('.tree-file').filter({hasText:'构建本地优先的知识库'}).first().click();await page.getByTitle('分屏',{exact:true}).click();await body.click();await page.keyboard.press('Control+End');await body.evaluate(el=>{const data=new DataTransfer();data.items.add(new File([new Uint8Array([1,2,3])],'pasted.png',{type:'image/png'}));el.dispatchEvent(new ClipboardEvent('paste',{bubbles:true,cancelable:true,clipboardData:data}))});await page.waitForFunction(()=>window.savedInputs.some(input=>input.content.includes('![pasted.png](https://cdn.example.test/paste.png)')));if(!(await page.evaluate(()=>window.lastImport.paste)))throw Error('Image paste bypassed PicGo flag');if(!await page.locator('.preview-pane').isVisible())throw Error('Paste changed split view');
// Clean up only unreferenced files, recheck at commit, and require an explicit
// choice before deleting references. Failed deletes keep the review open.
await page.getByRole('tab',{name:'附件库',exact:true}).click();const cleanup=page.getByRole('button',{name:'清理未引用附件',exact:true});await cleanup.waitFor();await page.waitForFunction(()=>!document.querySelector('[aria-label="清理未引用附件"]').disabled);
await page.screenshot({path:'docs/preview-attachment-cleanup-v0.12.4.png'});
await cleanup.click();const deletion=page.locator('.attachment-delete-modal');await deletion.waitFor();await page.evaluate(()=>window.binUsage=1);await deletion.getByRole('button',{name:'确认删除',exact:true}).click();await deletion.waitFor({state:'hidden'});if(await page.locator('.attachment-row').count()!==3)throw Error('Cleanup removed a newly referenced attachment');
await page.evaluate(()=>window.binUsage=0);await page.getByRole('tab',{name:'导航',exact:true}).click();await page.getByRole('tab',{name:'附件库',exact:true}).click();await page.waitForFunction(()=>!document.querySelector('[aria-label="清理未引用附件"]').disabled);await cleanup.click();await deletion.waitFor();await page.evaluate(()=>window.failDelete=true);await deletion.getByRole('button',{name:'确认删除',exact:true}).click();await deletion.getByRole('alert').filter({hasText:'模拟删除失败'}).waitFor();if(await page.locator('.attachment-row').count()!==3)throw Error('Failed delete removed attachments');await deletion.getByRole('button',{name:'确认删除',exact:true}).click();await deletion.waitFor({state:'hidden'});await page.waitForFunction(()=>document.querySelectorAll('.attachment-row').length===2);
await page.locator('.attachment-open').filter({hasText:'配置.json'}).click();await page.getByRole('button',{name:'删除附件 配置.json',exact:true}).click();await deletion.waitFor();await deletion.getByRole('button',{name:'取消',exact:true}).click();if(await page.locator('.attachment-row').count()!==2)throw Error('Cancel deleted an attachment');
await page.getByRole('checkbox',{name:'全选当前附件',exact:true}).check();await page.getByRole('button',{name:'删除选中（2）',exact:true}).click();await deletion.waitFor();await deletion.getByRole('checkbox',{name:'允许删除仍被引用的附件',exact:true}).check();await page.screenshot({path:'docs/preview-attachment-delete-v0.12.4.png'});await deletion.getByRole('button',{name:'确认删除',exact:true}).click();await deletion.waitFor({state:'hidden'});await page.waitForFunction(()=>document.querySelectorAll('.attachment-row').length===0);if(await page.locator('.attachment-content').count())throw Error('Deleted attachment preview remained open');if(!(await page.evaluate(()=>window.attachmentDeletes.at(-1).force)))throw Error('Forced deletion choice lost');
if(errors.length)throw Error(errors.join('\n'));console.log('PASS cleanup/reference recheck/failure/cancel/explicit force/bulk delete/preview clear; natural-size/fit/zoom/rotate/resize/minimize images; attachment switches/defaults/persistence/PicGo paste URL/light-dark themes; hidden metadata and title preserved on save, timestamp, toolbar selection, safe text attachment dialog/download, compact attachment list, default collapsed tree, image download and blank close from library/note without click-through');
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
