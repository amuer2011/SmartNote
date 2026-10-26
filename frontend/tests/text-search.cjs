const fs=require('fs'),vm=require('vm'),ts=require('typescript'),assert=require('node:assert/strict');
const moduleObject={exports:{}};vm.runInNewContext(ts.transpileModule(fs.readFileSync('src/services/text-search.ts','utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,{exports:moduleObject.exports,RegExp});
const {textMatches:m,applyMatches:a}=moduleObject.exports;
assert.equal(m('a.b axb','a.b').length,1);assert.equal(m('Hello hello','hello').length,2);assert.equal(m('Hello hello','hello',false,true).length,1);
assert.equal(a('abc12 def34',m('abc12 def34','([a-z]+)(\\d+)',true,false,'$2-$1')),'12-abc 34-def');
assert.equal(a('你好你好',m('你好你好','你好',false,false,'')),'');
assert.equal(a('abc',m('abc','(?=b)',true,false,'-')),'a-bc');
assert.equal(a('ab',m('ab','(?<word>a)',true,false,'$<word>$$')),'a$b');
assert.throws(()=>m('x','[',true));console.log('PASS: literal escaping, case sensitivity, regex groups, deletion, zero-width and invalid regex');
