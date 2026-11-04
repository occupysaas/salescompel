import { readFileSync } from 'node:fs';
import assert from 'node:assert/strict';
import type { Outcome, SourceBundle } from '../types.ts';
export function fixture(): SourceBundle { const root = new URL('../testdata/accountCard/', import.meta.url); return { name:'accountCard', html:readFileSync(new URL('accountCard.html',root),'utf8'), js:readFileSync(new URL('accountCard.js',root),'utf8'), metadata:readFileSync(new URL('accountCard.js-meta.xml',root),'utf8') }; }
export function good<T>(out:Outcome<T>):T {if(!out.ok)assert.fail(out.error.message); return out.value;}
export function bad<T>(out:Outcome<T>,code?:string){assert.equal(out.ok,false);if(out.ok)throw Error('accepted invalid input');if(code)assert.equal(out.error.code,code);assert(!out.error.message.includes('PRIVATE_MARKER'));return out.error;}
