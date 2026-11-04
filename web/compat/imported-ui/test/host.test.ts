import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createStore, bindRecord } from '../host.ts';
import { defaultLimits } from '../limits.ts';
import { good, bad } from './helpers.ts';
export const rows = [{
        tenant: 'a', id: '1', name: 'First'
    }, {
        tenant: 'b', id: '1', name: 'Second'
    }, {
        tenant: 'a', id: '2', name: 'Unrelated'
    }];
const identity = { tenant: 'a', principal: 'writer' };
const policy = {
    identity, readAllowed: true, nameWriteAllowed: true
};
test('hostStoreValidation', () => {
    const input = structuredClone(rows);
    const s = good(createStore(input));
    input[0].name = 'changed';
    const snapshot = s.snapshot() as any;
    snapshot[0].name = 'alias';
    assert.equal(s.snapshot()[0].name, 'First');
    for (const rs of [[{
                tenant: '', id: '1', name: 'x'
            }], [rows[0], rows[0]], [{
                tenant: 'a', id: '1', name: '\ud800'
            }]])
        bad(createStore(rs));
});
test('recordBoundReadAndSave', () => {
    const s = good(createStore(rows));
    const owner = {};
    const id = { ...identity };
    const p = { ...policy, identity: { ...identity } };
    const b = good(bindRecord(s, id, p, '1', owner, defaultLimits()));
    id.tenant = 'b';
    p.nameWriteAllowed = false;
    assert.deepEqual(good(b.load(owner)), { recordId: '1', name: 'First' });
    good(b.receive(owner, { type: 'save', detail: 'Changed' }));
    assert.deepEqual(s.snapshot(), [{
            tenant: 'a', id: '1', name: 'Changed'
        }, rows[2], rows[1]]);
});
test('rejectHostCapabilities', () => {
    for (const kind of ['owner', 'closed', 'type', 'extra', 'object', 'large', 'unicode', 'read', 'write', 'missing']) {
        const s = good(createStore(rows));
        const owner = {};
        const p = {
            ...policy, readAllowed: kind !== 'read', nameWriteAllowed: kind !== 'write'
        };
        const b = good(bindRecord(s, identity, p, kind === 'missing' ? 'absent' : '1', owner, defaultLimits()));
        if (kind === 'closed')
            b.close();
        let event: any = { type: 'save', detail: 'Changed' };
        if (kind === 'type')
            event.type = 'delete';
        if (kind === 'extra')
            event.tenant = 'b';
        if (kind === 'object')
            event.detail = { name: 'x' };
        if (kind === 'large')
            event.detail = 'x'.repeat(4097);
        if (kind === 'unicode')
            event.detail = '\ud800';
        if (kind === 'read' || kind === 'missing')
            bad(b.load(owner), kind === 'read' ? 'denied' : 'not-found');
        else
            bad(b.receive(kind === 'owner' ? {} : owner, event));
        assert.deepEqual(s.snapshot(), [rows[0], rows[2], rows[1]]);
    }
});
