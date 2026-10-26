const assert=require('node:assert/strict'),path=require('node:path'),{chromium}=require('playwright');
(async()=>{const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});try{
 const page=await browser.newPage({viewport:{width:1250,height:850}}),errors=[];page.on('pageerror',error=>errors.push(String(error)));
 await page.goto((process.env.SMARTNOTE_TEST_URL||'http://127.0.0.1:4175')+'/tests/admonition-completion.html');
 const editor=page.locator('.cm-content'),menu=page.locator('.cm-admonition-completion');await editor.waitFor();
 const content=()=>page.evaluate(()=>window.fixture.content.value);
 async function set(text){await page.evaluate(text=>{window.fixture.content.value=text},text);await editor.click();await editor.press('Control+End')}
 await editor.pressSequentially('```ad-');await menu.waitFor();
 assert.deepEqual(await menu.locator('.cm-completionLabel').allTextContents(),['note','seealso','abstract','summary','tldr','info','todo','tip','hint','important','success','check','done','question','help','faq','warning','caution','attention','failure','fail','missing','danger','error','bug','example','quote','cite']);
 assert.equal(await menu.locator('.cm-admonition-icon svg').count(),28);assert.equal(await menu.getByRole('option').first().getAttribute('aria-selected'),'true');
 const bounds=await menu.boundingBox();assert(bounds.width<230&&bounds.height<310);assert(await menu.locator('ul').evaluate(el=>el.scrollHeight>el.clientHeight));
 await page.evaluate(()=>document.documentElement.dataset.theme='dark');await page.screenshot({path:path.resolve(__dirname,'../../docs/preview-admonition-completion-dark-v0.12.7.png')});
 const dark=await menu.evaluate(el=>getComputedStyle(el).backgroundColor);await page.evaluate(()=>document.documentElement.dataset.theme='light');assert.notEqual(await menu.evaluate(el=>getComputedStyle(el).backgroundColor),dark);
 await page.screenshot({path:path.resolve(__dirname,'../../docs/preview-admonition-completion-light-v0.12.7.png')});
 await editor.press('ArrowDown');await editor.press('Enter');await menu.waitFor({state:'hidden'});assert.equal(await content(),'```ad-seealso\n\n```');
 await editor.press('Control+z');assert.equal(await content(),'```ad-\n\n```');
 await editor.press('Control+Space');await menu.waitFor();await menu.locator('.cm-completionLabel').filter({hasText:/^warning$/}).click();assert.equal(await content(),'```ad-warning\n\n```');
 await set('```ad-');await editor.pressSequentially('ti');await menu.waitFor();assert.deepEqual(await menu.locator('.cm-completionLabel').allTextContents(),['tip']);await editor.press('Tab');assert.equal(await content(),'```ad-tip');
 await set('> [!');await editor.press('Control+Space');await menu.waitFor();await editor.pressSequentially('danger');await menu.locator('.cm-completionLabel').filter({hasText:/^danger$/}).waitFor();await editor.press('Enter');assert.equal(await content(),'> [!danger]');
 await editor.press('Enter');await editor.pressSequentially('> 内容');await page.locator('.markdown-preview .admonition-danger').waitFor();
 await set('```ad-note\ntitle: 原有标题\n正文\n```');await page.evaluate(()=>window.fixture.editor.value.jumpToPosition(0,6));await editor.press('Control+Space');await menu.waitFor();await menu.locator('.cm-completionLabel').filter({hasText:/^note$/}).click();assert.equal(await content(),'```ad-note\ntitle: 原有标题\n正文\n```');
 await set('> [!note]- 原有标题\n> 正文');await page.evaluate(()=>window.fixture.editor.value.jumpToPosition(0,6));await editor.press('Control+Space');await menu.waitFor();await menu.locator('.cm-completionLabel').filter({hasText:/^note$/}).click();assert.equal(await content(),'> [!note]- 原有标题\n> 正文');
 await set('> ```ad-');await editor.press('Control+Space');await menu.waitFor();await editor.press('Tab');assert.equal(await content(),'> ```ad-note');
 await set('~~~ad-');await editor.press('Control+Space');await menu.waitFor();await editor.press('Escape');await menu.waitFor({state:'hidden'});assert.equal(await content(),'~~~ad-');
 for(const text of ['正文 ad-','`ad-','```js\nad-','````js\n```ad-','    ```ad-','```md\n> [!','\\```ad-']){await set(text);await editor.press('Control+Space');await page.waitForTimeout(150);assert.equal(await menu.count(),0,'Unexpected completion in '+text)}
 await set('```ad-');await editor.pressSequentially('zzz');await page.waitForTimeout(150);assert.equal(await menu.count(),0);
 await set('---\ntitle: 提示块\n---\n\n```ad-');await editor.press('Control+Space');await menu.waitFor();await editor.press('Tab');assert.equal(await content(),'---\ntitle: 提示块\n---\n\n```ad-note');
 await page.setViewportSize({width:700,height:500});await set('\n'.repeat(45)+'```ad-');await editor.press('Control+Space');await menu.waitFor();const small=await menu.boundingBox();assert(small.x>=0&&small.y>=0&&small.x+small.width<=701&&small.y+small.height<=501);
 assert.deepEqual(errors,[]);console.log('PASS Admonition menu order/colors/icons/themes, prefix filtering, arrows/Enter/Tab/Escape/mouse, undo, original title/body/metadata preservation, Callouts, quoted/tilde fences, code/inline-code exclusion and viewport fit');
}finally{await browser.close()}})().catch(error=>{console.error(error);process.exit(1)});
