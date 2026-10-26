const assert=require('node:assert/strict'),fs=require('node:fs'),ts=require('typescript'),Module=require('node:module');
const code=ts.transpileModule(fs.readFileSync('frontend/src/services/editor-document.ts','utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText;const mod=new Module('editor-document');mod._compile(code,'editor-document.js');const {editorDocument,noteTimestamp}=mod.exports;
const original='---\nid: stable\ntitle: "标题"\n---\n\n# 标题\n\n正文\n## 小节\n内容';
const parts=editorDocument(original,'标题');assert.equal(parts.prefix+parts.body,original);assert.equal(parts.body,'\n正文\n## 小节\n内容');assert.equal(parts.lineOffset,6);assert.equal(original.split('\n')[parts.lineOffset+2],'## 小节');
assert.equal(editorDocument(original,'标题',true).body,original);assert.equal(editorDocument('# 普通标题\n正文','普通标题').prefix,'');assert(editorDocument(original,'其他标题').body.includes('# 标题'));
const crlf=original.replace(/\n/g,'\r\n');assert.equal(editorDocument(crlf,'标题').prefix+editorDocument(crlf,'标题').body,crlf);
assert.equal(editorDocument('---\nid: stable\n---','标题').prefix,'---\nid: stable\n---\n');
assert.match(noteTimestamp('2026-10-19T15:35:14+08:00'),/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/);
console.log('PASS metadata/title visibility, source offsets, CRLF preservation, EOF body boundary and timestamp format');

