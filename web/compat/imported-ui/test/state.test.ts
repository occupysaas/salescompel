import { test } from 'node:test';
import assert from 'node:assert/strict';
import { compileBundle } from '../compile.ts';
import { defaultLimits, createMeter } from '../limits.ts';
import { initialize, prepareAction, prepareProps } from '../state.ts';
import { fixture, good, bad } from './helpers.ts';
const program = () => good(compileBundle(fixture(), defaultLimits()));
const meter = () => good(createMeter(defaultLimits()));
test('initializePrivateState', () => { const p = program(); const props = { recordId: '1', name: 'First' }; const s = good(initialize(p, props, meter())); assert.equal(s.values.draft, 'First'); props.name = 'later'; assert.equal(s.values.name, 'First'); for (const props of [{ recordId: '1' }, { recordId: '1', name: 'x', draft: 'bad' }, { recordId: '1', name: null }])
    bad(initialize(p, props as any, meter())); });
test('prepareOrderedAction', () => { const s = good(initialize(program(), { recordId: '1', name: 'First' }, meter())); const a = good(prepareAction(s, 'handleInput', 'Draft', meter())); assert.equal(a.next.values.draft, 'Draft'); assert.equal(s.values.draft, 'First'); const saved = good(prepareAction(a.next, 'handleSave', undefined, meter())); assert.deepEqual(saved.events, [{ type: 'save', detail: 'Draft' }]); const patch = good(prepareProps(a.next, { name: 'Server' }, meter())); assert.equal(patch.next.values.name, 'Server'); assert.equal(patch.next.values.draft, 'Draft'); for (const p of [{ draft: 'x' }, { name: null }])
    bad(prepareProps(s, p as any, meter())); });
test('abortPreparedAction', () => { const s = good(initialize(program(), { recordId: '1', name: 'First' }, meter())); const c = new AbortController(); let count = 0; const m = good(createMeter(defaultLimits(), c.signal, () => { if (++count === 3)
    c.abort(); })); bad(prepareAction(s, 'handleInput', 'Changed', m), 'cancelled'); assert.equal(s.values.draft, 'First'); });
test('exhaustPreparedAction', () => { const b = fixture(); const two = b.js.replace("this.dispatchEvent(new CustomEvent('save', { detail: this.draft }));", "this.draft = 'changed'; this.dispatchEvent(new CustomEvent('save', { detail: this.draft })); this.dispatchEvent(new CustomEvent('save', { detail: this.draft }));"); const s = good(initialize(good(compileBundle({ ...b, js: two }, defaultLimits())), { recordId: '1', name: 'First' }, meter())); bad(prepareAction(s, 'handleSave', undefined, meter()), 'budget'); assert.equal(s.values.draft, 'First'); bad(prepareAction(s, 'handleInput', 'x'.repeat(4097), meter()), 'budget'); bad(prepareAction(s, 'handleInput', 'x', good(createMeter({ ...defaultLimits(), steps: 1 }))), 'budget'); });
