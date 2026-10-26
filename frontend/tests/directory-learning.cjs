const {chromium}=require('playwright');
(async()=>{const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});try{
 const page=await browser.newPage({viewport:{width:1450,height:1100}}),errors=[];page.on('pageerror',e=>errors.push(String(e)));
 await page.addInitScript(()=>{
  const workspace={id:'w',name:'学习测试',path:'D:/IsolatedLearning'};
  const notes=Array.from({length:24},(_,i)=>({id:'n'+i,title:'目录样本'+i,path:'案例/目录样本'+i+'.md',content:'# 目录样本'+i,hash:'hash'+i,tags:[],project:'',updated:new Date().toISOString()}));
  const defaults={revision:'r0',model:'BAAI/bge-small-zh-v1.5',automatic:false,automaticVersion:1,threshold:.55,categories:[{id:'a',name:'案例学习',description:'原始分类说明',directory:'案例',enabled:true},{id:'b',name:'另一分类',description:'另一个说明',directory:'其他',enabled:true}]};
  const config=()=>JSON.parse(localStorage.getItem('learning-config')||JSON.stringify(defaults));
  window.previewCalls=[];
  const app={Snapshot:async()=>({settings:{activeId:'w',workspaces:[workspace],theme:'light',picGoUrl:'',ui:{fontSize:14,collapsedWorkspaces:[],entryOrder:{}}},notes,directories:['案例','其他'],tasks:[],attachments:[]}),DataDirectory:async()=> 'D:/IsolatedLearningData',GetBookmarks:async()=>[],DueReminders:async()=>[],Graph:async()=>[],ListWorkspaceFiles:async()=>({notes,directories:['案例','其他']}),GetHolidayYear:async year=>({year,holiday:{},type:{}}),GetLunarYear:async year=>({year,version:1,days:{}}),
   EmbeddingStatus:async()=>({state:'ready',message:'离线模型已就绪',dimensions:512}),GetSemanticConfig:async()=>config(),GetArchiveRules:async()=>({revision:'r0',rules:[]}),
   SaveSemanticConfig:async(id,value)=>{if(window.rejectSave)throw Error('学习样本已修改，请重新选择');if(value.revision!==config().revision)throw Error('分类配置已变化');const next={...structuredClone(value),revision:'r'+Date.now()};localStorage.setItem('learning-config',JSON.stringify(next));return next},
   PreviewDirectoryLearning:async(id,category)=>{window.previewCalls.push(category);const delay=window.delayNext||0;window.delayNext=0;if(delay)await new Promise(r=>setTimeout(r,delay));return {directory:category==='a'?'案例':'其他',total:category==='a'?notes.length:0,notes:category==='a'?notes.map(n=>({noteId:n.id,title:n.title,path:n.path,hash:n.hash})):[]}},
   SetUIPreferences:async()=>{},AutoArchive:async()=>null
  };window.go={main:{App:app}};
 });
 await page.goto(process.env.SMARTNOTE_TEST_URL||'http://127.0.0.1:4175');
 async function settings(){await page.getByRole('button',{name:'打开设置窗口',exact:true}).click();await page.locator('.settings-nav').getByRole('button',{name:'智能归档',exact:true}).click();await page.locator('.semantic-settings article').first().waitFor();}
 await settings();const cards=page.locator('.semantic-settings article');
 const openA=()=>page.getByRole('button',{name:'学习目录 案例学习',exact:true}).click();
 const panel=page.locator('.directory-learning'),dialog=page.locator('.n-card').filter({has:panel});
 await openA();await panel.getByRole('checkbox',{name:'选择样本 目录样本23',exact:true}).waitFor();
 await panel.getByRole('button',{name:'选择前20篇',exact:true}).click();
 if(await panel.locator('input[type=checkbox]:checked').count()!==20)throw Error('Sample cap failed');
 if(!await panel.getByRole('checkbox',{name:'选择样本 目录样本23',exact:true}).isDisabled())throw Error('21st sample enabled');
 await dialog.getByRole('button',{name:'确认学习样本',exact:true}).click();await panel.waitFor({state:'hidden'});
 await cards.first().getByText('已保存 20 篇学习样本',{exact:true}).waitFor();
 await page.reload();await settings();await openA();await panel.getByRole('checkbox',{name:'选择样本 目录样本23',exact:true}).waitFor();
 if(await panel.locator('input[type=checkbox]:checked').count()!==20)throw Error('Reload lost saved selections');
 await panel.getByRole('button',{name:'取消选择',exact:true}).click();await panel.getByRole('textbox',{name:'筛选学习样本'}).fill('目录样本23');
 await panel.getByRole('checkbox',{name:'选择样本 目录样本23',exact:true}).check();
 await page.evaluate(()=>window.rejectSave=true);await dialog.getByRole('button',{name:'确认学习样本',exact:true}).click();await panel.getByRole('alert').filter({hasText:'学习样本已修改'}).waitFor();
 if(await page.evaluate(()=>JSON.parse(localStorage.getItem('learning-config')).categories[0].examples.length)!==20)throw Error('Failed save changed references');
 await page.evaluate(()=>window.rejectSave=false);await dialog.getByRole('button',{name:'确认学习样本',exact:true}).click();await panel.waitFor({state:'hidden'});
 await cards.first().getByText('已保存 1 篇学习样本',{exact:true}).waitFor();
 await openA();await panel.getByRole('checkbox',{name:'选择样本 目录样本23',exact:true}).waitFor();await dialog.getByRole('button',{name:'清除学习',exact:true}).click();await panel.waitFor({state:'hidden'});
 if(await page.evaluate(()=>JSON.parse(localStorage.getItem('learning-config')).categories[0].examples.length)!==0)throw Error('Clear did not persist');
 await cards.first().getByRole('textbox',{name:'分类说明',exact:true}).fill('尚未保存的说明');await openA();
 await page.locator('.semantic-settings').getByRole('alert').filter({hasText:'请先保存当前分类修改'}).waitFor();if(await panel.isVisible())throw Error('Unsaved draft was discarded');
 await cards.first().getByRole('textbox',{name:'分类说明',exact:true}).fill('原始分类说明');
 await page.evaluate(()=>window.delayNext=1000);await openA();await panel.getByText('正在读取目录样本…',{exact:true}).waitFor();await dialog.getByRole('button',{name:'取消',exact:true}).click();
 await page.getByRole('button',{name:'学习目录 另一分类',exact:true}).click();await panel.getByText('此目录暂无可学习的直属笔记。',{exact:true}).waitFor();await page.waitForTimeout(1200);
 if(await panel.locator('input[type=checkbox]').count())throw Error('Closed preview contaminated another category');
 if(errors.length)throw Error(errors.join('\n'));
 console.log('PASS directory samples: selection cap, filtering, confirmation, persistence, conflict protection, clearing, draft preservation, stale preview rejection');
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
