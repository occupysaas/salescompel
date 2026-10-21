import test from 'node:test';import assert from 'node:assert/strict';
import {parseGateEvents} from '../src/candidate-acceptance-results.ts';
test('native event adapters require terminal evidence and preserve failures and skips',()=>{
 assert.throws(()=>parseGateEvents('node','', 'lab'));
 const events=[{type:'test:pass',data:{name:'sample',file:'/tmp/core.test.ts'}},{type:'test:summary',data:{}}].map(x=>JSON.stringify(x)).join('\n');
 assert.deepEqual(parseGateEvents('node',events,'lab'),[{id:'core.test.ts::sample',outcome:'passed'}]);
 assert.throws(()=>parseGateEvents('go',JSON.stringify({Action:'run',Package:'p',Test:'T'}),'database'));
 assert.throws(()=>parseGateEvents('playwright','{}','preview'));
});
