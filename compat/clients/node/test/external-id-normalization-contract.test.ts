import assert from 'node:assert/strict';
import test from 'node:test';
import { normalizationManifest, normalizationManifestHash, normalizationScope, normalizationRequest, validateNormalizationReport } from '../src/external-id-normalization-contract.ts';
test('fixed normalization matrix retains exact directional inputs and bounds', () => {
  assert.equal(normalizationManifest.length, 36);
  assert.equal(normalizationManifest.reduce((n,c)=>n+c.steps.length,0),80);
  assert.equal(normalizationManifest.flatMap(c=>c.steps).filter(s=>s.kind!=='update').length,64);
  assert.equal(new Set(normalizationManifest.map(c=>c.id)).size,36);
  assert.match(normalizationManifestHash,/^[a-f0-9]{64}$/);
  assert.ok(Object.isFrozen(normalizationManifest));
  const run='a'.repeat(24);
  for(const c of normalizationManifest) {
    assert.ok(Object.isFrozen(c.steps));
    const scope=normalizationScope(run,c);
    assert.ok(Object.values(scope.keys).every(k=>k.length<=80));
    for(const s of c.steps) {
      const r=normalizationRequest(run,c,s,{first:'001000000000000001',second:'001000000000000002'});
      assert.equal(r.key,scope.keys[s.key]);
      assert.equal(r.body.Name,scope.names[s.id]);
      assert.equal(Object.hasOwn(r.body,'Compat_Key__c'),s.kind!=='upsert');
      if(s.kind==='upsert') assert.ok(r.path.endsWith(encodeURIComponent(r.key)));
    }
  }
  const first=normalizationScope(run,normalizationManifest[0]);
  assert.equal(first.keys.b,' '+first.keys.a);
  const reverse=normalizationScope(run,normalizationManifest.find(c=>c.id==='leading-space-reverse-lookup')!);
  assert.equal(reverse.keys.a,' '+reverse.keys.b);
  const accent=normalizationScope(run,normalizationManifest.find(c=>c.id==='accent-create')!);
  assert.ok(accent.keys.b.endsWith('e\u0301'));
  assert.throws(()=>normalizationScope('../bad',normalizationManifest[0]));
  assert.throws(()=>normalizationRequest(run,normalizationManifest.find(c=>c.id==='ascii-same-update')!,{id:'request',kind:'update',key:'b',target:'first'},{}));
});
test('report contract rejects malformed and foreign evidence',()=>{
  for(const value of [null,{}, {schemaVersion:1,kind:'external-id-body-validation'}]) assert.throws(()=>validateNormalizationReport(value));
});
