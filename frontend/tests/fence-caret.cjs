const assert=require('node:assert/strict'),{chromium}=require('playwright');
(async()=>{const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});try{
 const page=await browser.newPage({viewport:{width:1250,height:850}}),errors=[];page.on('pageerror',error=>errors.push(String(error)));
 await page.goto((process.env.SMARTNOTE_TEST_URL||'http://127.0.0.1:4175')+'/tests/admonition-completion.html');const editor=page.locator('.cm-content');await editor.waitFor();
 const content=()=>page.evaluate(()=>window.fixture.content.value);
 async function set(text){await page.evaluate(text=>window.fixture.content.value=text,text);await editor.click();await editor.press('Control+End')}
 await editor.pressSequentially('```');assert.equal(await content(),'```\n\n```');await editor.pressSequentially('python');assert.equal(await content(),'```python\n\n```');await editor.press('Enter');await editor.pressSequentially('print(1)');assert.equal(await content(),'```python\nprint(1)\n```');
 await set('');await editor.pressSequentially('```');await editor.press('Control+z');assert(!(await content()).includes('```'),'Undo left an orphan closing fence');
 await set('> ');await editor.pressSequentially('```');assert.equal(await content(),'> ```\n> \n> ```');await editor.press('Enter');await editor.pressSequentially('quoted');assert.equal(await content(),'> ```\n> quoted\n> ```');
 for(const [initial,expected] of [['正文 ', '正文 ```'],['    ','    ```'],['```js\n','```js\n```']]){await set(initial);await editor.pressSequentially('```');assert.equal(await content(),expected)}
 await set('``\n\n```');await page.evaluate(()=>window.fixture.editor.value.jumpToPosition(0,2));await editor.pressSequentially('`');assert.equal(await content(),'```\n\n```');
 await set('');await page.keyboard.insertText('```js\nconst x = 1\n```');assert.equal(await content(),'```js\nconst x = 1\n```');
 await set('');await editor.pressSequentially('```ad-');await page.locator('.cm-admonition-completion').waitFor();await editor.press('Tab');assert.equal(await content(),'```ad-note\n\n```');await editor.press('Enter');await editor.pressSequentially('提示正文');assert.equal(await content(),'```ad-note\n提示正文\n```');await page.locator('.markdown-preview .admonition').waitFor();
 await page.evaluate(()=>document.documentElement.dataset.theme='dark');await editor.focus();await page.locator('.cm-cursor').waitFor();assert.equal(await page.locator('.cm-cursor').first().evaluate(el=>getComputedStyle(el).borderLeftColor),'rgb(255, 255, 255)');await page.screenshot({path:'docs/preview-fence-caret-v0.12.18.png',animations:'disabled'});
 await page.evaluate(()=>document.documentElement.dataset.theme='light');assert.notEqual(await page.locator('.cm-cursor').first().evaluate(el=>getComputedStyle(el).borderLeftColor),'rgb(255, 255, 255)');assert.deepEqual(errors,[]);console.log('PASS typed fence pair/language/Enter/undo/quote/no duplicates/code exclusion/bulk text/admonition compatibility; white dark caret and readable light caret');
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
