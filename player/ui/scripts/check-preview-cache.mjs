// Node-only regression of the browser cache; no window is opened.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import ts from 'typescript';
const transpile=(path)=>ts.transpileModule(readFileSync(new URL(path,import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const moduleURL=(source)=>'data:text/javascript;base64,'+Buffer.from(source).toString('base64');
let calls=0,active=0,maximum=0,payload='AA==',delay=2;
let waitForRelease=null;
globalThis.window={__TAURI__:{core:{invoke:async()=>{
 calls++; active++; maximum=Math.max(maximum,active);
 if(waitForRelease) await waitForRelease;
 else await new Promise(resolve=>setTimeout(resolve,delay));
 active--; return JSON.stringify({state:'ready',data_url:'data:video/mp4;base64,'+payload});
}}}};
let created=0,revoked=0;
const create=URL.createObjectURL.bind(URL),revoke=URL.revokeObjectURL.bind(URL);
URL.createObjectURL=(blob)=>{created++;return create(blob);};
URL.revokeObjectURL=(url)=>{revoked++;return revoke(url);};
const bridge=moduleURL(transpile('../src/lib/bridge.ts'));
const cache=await import(moduleURL(transpile('../src/lib/previewCache.ts').replace("'./bridge'",JSON.stringify(bridge))));
try {
 const results=await Promise.all(Array.from({length:20},(_,i)=>cache.preview('movie',i+1)));
 assert.equal(maximum,2);assert.equal(results.length,20);assert.equal(cache.previewUsage().clips,12);assert.equal(revoked,8);
 const before=calls;await Promise.all([cache.preview('episode',42),cache.preview('episode',42)]);assert.equal(calls-before,1);
 cache.clearPreviews();assert.equal(created,revoked);
 payload=Buffer.alloc(8*1024*1024).toString('base64');
 await Promise.all(Array.from({length:4},(_,i)=>cache.preview('movie',100+i)));
 assert.ok(cache.previewUsage().bytes<=24*1024*1024);assert.equal(cache.previewUsage().clips,3);
 cache.clearPreviews();payload='AA==';let release;waitForRelease=new Promise(resolve=>{release=resolve;});
 const late=cache.preview('movie',999);await new Promise(resolve=>setTimeout(resolve,5));cache.clearPreviews();release();
 assert.deepEqual(await late,{});assert.equal(cache.previewUsage().clips,0);assert.equal(created,revoked);
 console.log('preview cache: concurrency 2, deduplication, 12 clips / 24 MiB, revoked blobs and stale scope passed');
}finally{cache.clearPreviews();URL.createObjectURL=create;URL.revokeObjectURL=revoke;}
