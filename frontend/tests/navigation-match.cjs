const fs=require('fs'),vm=require('vm'),ts=require('typescript'),assert=require('node:assert/strict');
const moduleObject={exports:{}};vm.runInNewContext(ts.transpileModule(fs.readFileSync('src/services/navigation-match.ts','utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,{exports:moduleObject.exports,RegExp});
const {navigationMatch:m,navigationHighlight:h}=moduleObject.exports;
const note={path:'folder/采购计划.md',title:'别名',content:'---\nid: hidden\n---\n\n很长的正文'.repeat(1)};
assert.ok(m(note,'采购'));assert.equal(m(note,'别名'),undefined);assert.equal(m(note,'hidden'),undefined);assert.equal(m(note,'folder'),undefined);assert.equal(m(note,'买东西'),undefined);
note.content='---\nid: hidden\n---\n\n'+ '前文'.repeat(100)+'AbC .*[ ] <script>关键字</script>'+'后文'.repeat(100);
const result=m(note,'abc');assert.equal(result.line,4);assert.ok(Array.from(result.snippet).length<=68);assert.ok(result.snippet.includes('AbC'));assert.equal(h(result.snippet,'abc').filter(x=>x.hit)[0].text,'AbC');
assert.ok(m(note,'.*['));assert.equal(m(note,'not found'),undefined);assert.ok(m(note,'<script>'));assert.equal(h('a.b axb','a.b').filter(x=>x.hit).length,1);
console.log('PASS navigation literal filename/body matching, metadata exclusion, Unicode snippets, bounded context, line offsets and safe highlight segments');
