import type { InstanceState, ActionPlan } from '../state.ts';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { scenarios, loadBundle, parseScenarios } from './scenario-data.ts';
import { compileBundle } from '../compile.ts';
import { createMeter } from '../limits.ts';
import { initialize, prepareAction, prepareProps } from '../state.ts';
import { createStore, bindRecord } from '../host.ts';
import { unwrap } from '../types.ts';
import { good } from './helpers.ts';
const hashes = new Map<string, string>();
for (const s of scenarios)
    test('scenario ' + s.id, () => {
        for (let repeat = 0; repeat < 2; repeat++) {
            const bundle = loadBundle(s.bundle);
            for (const file of ['html', 'js', 'metadata'] as const) {
                const key = s.bundle + '.' + file;
                const hash = createHash('sha256').update(bundle[file]).digest('hex');
                if (hashes.has(key))
                    assert.equal(hash, hashes.get(key));
                hashes.set(key, hash);
            }
            const store = good(createStore(s.records));
            let diagnostic: string | null = null;
            let state: InstanceState | undefined;
            const events: string[] = [];
            try {
                const p = unwrap(compileBundle(bundle, s.limits));
                const owner = {};
                const binding = unwrap(bindRecord(store, s.identity, s.policy, s.recordId, owner, s.limits));
                state = unwrap(initialize(p, unwrap(binding.load(owner)), unwrap(createMeter(s.limits))));
                for (const a of s.actions) {
                    try {
                        const m = unwrap(createMeter(s.limits));
                        const plan: ActionPlan = unwrap(a.type === 'props' ? prepareProps(state, a.value, m) : prepareAction(state, a.type === 'input' ? 'handleInput' : 'handleSave', a.type === 'input' ? a.value : undefined, m));
                        state = plan.next;
                        for (const e of plan.events) {
                            events.push(e.type);
                            unwrap(binding.receive(owner, e));
                        }
                        ;
                        diagnostic = null;
                    }
                    catch (e: any) {
                        diagnostic = e.code;
                    }
                }
                binding.close();
            }
            catch (e: any) {
                diagnostic = e.code;
            }
            assert.equal(diagnostic, s.expected.diagnostic, s.id);
            assert.deepEqual(events, s.expected.events);
            assert.deepEqual(store.snapshot(), s.expected.store);
            if (s.expected.input !== null)
                assert.equal(state?.values.draft, s.expected.input);
        }
    });
test('scenario schema rejects unknown duplicate unsafe and malformed input', () => {
    for (const modify of [(c: any) => c.extra = true, (c: any) => c.bundle = '../escape', (c: any) => c.actions = [{ type: 'eval' }], (c: any) => c.limits.steps = 0, (c: any) => c.sourceRefs = ['unknown'], (c: any) => c.expected.events = ['delete']]) {
        const rows = structuredClone(scenarios);
        modify(rows[0]);
        assert.throws(() => parseScenarios(JSON.stringify(rows)));
    }
    assert.throws(() => parseScenarios(JSON.stringify([scenarios[0], scenarios[0]])));
    assert.throws(() => parseScenarios(JSON.stringify(scenarios) + '{}'));
});
