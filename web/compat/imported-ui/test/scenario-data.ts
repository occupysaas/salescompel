import { readFileSync } from 'node:fs';
import { createMeter } from '../limits.ts';
import { createStore } from '../host.ts';
import type { Identity, Policy, Account } from '../host.ts';
import type { Limits, SourceBundle } from '../types.ts';
export interface Scenario {
    id: string;
    bundle: string;
    records: Account[];
    identity: Identity;
    policy: Policy;
    recordId: string;
    limits: Limits;
    actions: ({
        type: 'input';
        value: string;
    } | {
        type: 'click';
    } | {
        type: 'props';
        value: Record<string, string>;
    })[];
    expected: {
        text: string | null;
        input: string | null;
        events: string[];
        store: Account[];
        diagnostic: string | null;
    };
    basis: string;
    sourceRefs: string[];
}
export const bundles = ['accountCard', 'doubleSave', 'unusedBad', 'badTemplate', 'badMetadata'];
function object(v: any, keys: string[]) {
    if (!v || typeof v !== 'object' || Array.isArray(v) || Object.keys(v).sort().join(',') !== keys.sort().join(','))
        throw Error('invalid scenario object');
}
function identity(v: any) {
    object(v, ['tenant', 'principal']);
    if (typeof v.tenant !== 'string' || !v.tenant || typeof v.principal !== 'string' || !v.principal)
        throw Error('invalid identity');
}
function records(v: any) {
    if (!Array.isArray(v))
        throw Error('invalid rows');
    for (const r of v) {
        object(r, ['tenant', 'id', 'name']);
        if ([r.tenant, r.id, r.name].some(v => typeof v !== 'string'))
            throw Error('invalid row');
    }
    if (!createStore(v).ok)
        throw Error('invalid records');
}
export function parseScenarios(text: string): Scenario[] {
    const cases = JSON.parse(text);
    if (!Array.isArray(cases) || !cases.length)
        throw Error('empty scenarios');
    const ids = new Set();
    for (const c of cases) {
        object(c, ['id', 'bundle', 'records', 'identity', 'policy', 'recordId', 'limits', 'actions', 'expected', 'basis', 'sourceRefs']);
        if (typeof c.id !== 'string' || !c.id || ids.has(c.id))
            throw Error('invalid id');
        ids.add(c.id);
        if (!bundles.includes(c.bundle))
            throw Error('invalid source');
        records(c.records);
        identity(c.identity);
        object(c.policy, ['identity', 'readAllowed', 'nameWriteAllowed']);
        identity(c.policy.identity);
        if (typeof c.policy.readAllowed !== 'boolean' || typeof c.policy.nameWriteAllowed !== 'boolean' || typeof c.recordId !== 'string' || !c.recordId)
            throw Error('invalid policy');
        object(c.limits, ['bundleBytes', 'fileBytes', 'tokens', 'nesting', 'fields', 'methods', 'statements', 'templateNodes', 'stringBytes', 'steps', 'events']);
        if (!createMeter(c.limits).ok)
            throw Error('invalid limits');
        if (!Array.isArray(c.actions))
            throw Error('invalid actions');
        for (const a of c.actions) {
            if (a.type === 'click')
                object(a, ['type']);
            else if (a.type === 'input') {
                object(a, ['type', 'value']);
                if (typeof a.value !== 'string')
                    throw Error('invalid value');
            }
            else if (a.type === 'props') {
                object(a, ['type', 'value']);
                if (!a.value || Array.isArray(a.value) || typeof a.value !== 'object' || Object.values(a.value).some(v => typeof v !== 'string'))
                    throw Error('invalid props');
            }
            else
                throw Error('invalid action');
        }
        ;
        object(c.expected, ['text', 'input', 'events', 'store', 'diagnostic']);
        if ([c.expected.text, c.expected.input].some(v => v !== null && typeof v !== 'string') || !Array.isArray(c.expected.events) || c.expected.events.some((v: any) => v !== 'save') || c.expected.diagnostic !== null && !['invalid-input', 'syntax', 'unsupported', 'invalid-model', 'type', 'denied', 'not-found', 'budget', 'cancelled', 'disposed', 'host'].includes(c.expected.diagnostic))
            throw Error('invalid expectation');
        records(c.expected.store);
        if (typeof c.basis !== 'string' || !c.basis || !Array.isArray(c.sourceRefs) || !c.sourceRefs.length || c.sourceRefs.some((v: any) => !['html', 'script', 'props', 'events', 'metadata', 'reactive', 'dom', 'input'].includes(v)))
            throw Error('invalid provenance');
    }
    ;
    return cases;
}
export function loadBundle(name: string): SourceBundle {
    if (!bundles.includes(name))
        throw Error('unsafe fixture path');
    const root = new URL('../testdata/' + name + '/', import.meta.url);
    return {
        name, html: readFileSync(new URL(name + '.html', root), 'utf8'), js: readFileSync(new URL(name + '.js', root), 'utf8'), metadata: readFileSync(new URL(name + '.js-meta.xml', root), 'utf8')
    };
}
export const scenarios = parseScenarios(readFileSync(new URL('../testdata/scenarios.json', import.meta.url), 'utf8'));
