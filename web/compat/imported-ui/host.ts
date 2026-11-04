import { attempt, bytes, fail, unwrap } from './types.ts';
import type { Limits, Outcome } from './types.ts';
import { createMeter } from './limits.ts';
import type { ComponentEvent } from './state.ts';
export type Identity = Readonly<{
    tenant: string;
    principal: string;
}>;
export type Policy = Readonly<{
    identity: Identity;
    readAllowed: boolean;
    nameWriteAllowed: boolean;
}>;
export type Account = Readonly<{
    tenant: string;
    id: string;
    name: string;
}>;
export interface SyntheticStore {
    snapshot(): readonly Account[];
}
export interface RecordBinding {
    load(owner: object): Outcome<Readonly<{
        recordId: string;
        name: string;
    }>>;
    receive(owner: object, event: ComponentEvent): Outcome<void>;
    close(): void;
}
const stores = new WeakMap<SyntheticStore, Map<string, Map<string, string>>>();
export function createStore(rows: readonly Account[]): Outcome<SyntheticStore> {
    return attempt(() => {
        if (!Array.isArray(rows))
            fail('invalid-input');
        const data = new Map<string, Map<string, string>>();
        for (const r of rows) {
            if (!r || typeof r.tenant !== 'string' || typeof r.id !== 'string' || typeof r.name !== 'string' || !r.tenant || !r.id)
                fail('invalid-input');
            for (const v of [r.tenant, r.id, r.name])
                try {
                    bytes(v);
                }
                catch {
                    fail('invalid-input');
                }
            let tenant = data.get(r.tenant);
            if (!tenant) {
                tenant = new Map();
                data.set(r.tenant, tenant);
            }
            if (tenant.has(r.id))
                fail('invalid-input');
            tenant.set(r.id, r.name);
        }
        const store = Object.freeze({ snapshot: () => {
                const result: Account[] = [];
                for (const [tenant, records] of data)
                    for (const [id, name] of records)
                        result.push({
                            tenant, id, name
                        });
                return result.sort((a, b) => a.tenant < b.tenant ? -1 : a.tenant > b.tenant ? 1 : a.id < b.id ? -1 : a.id > b.id ? 1 : 0);
            } });
        stores.set(store, data);
        return store;
    });
}
export function bindRecord(store: SyntheticStore, id: Identity, policy: Policy, recordId: string, owner: object, limits: Limits): Outcome<RecordBinding> {
    return attempt(() => {
        const m = unwrap(createMeter(limits));
        const data = stores.get(store);
        if (!data || !id || !policy || !policy.identity || !owner || typeof owner !== 'object')
            fail('invalid-input');
        for (const value of [id.tenant, id.principal, recordId])
            if (!m.string(value))
                fail('invalid-input');
        if (id.tenant !== policy.identity.tenant || id.principal !== policy.identity.principal)
            fail('denied', 'host');
        if (typeof policy.readAllowed !== 'boolean' || typeof policy.nameWriteAllowed !== 'boolean')
            fail('invalid-input');
        const context = {
            tenant: id.tenant, recordId, read: policy.readAllowed, write: policy.nameWriteAllowed
        };
        let closed = false;
        const check = (caller: object) => {
            if (closed)
                fail('disposed', 'host');
            if (caller !== owner)
                fail('denied', 'host');
        };
        return Object.freeze({
            load(caller: object) {
                return attempt(() => {
                    check(caller);
                    if (!context.read)
                        fail('denied', 'host');
                    const name = data.get(context.tenant)?.get(context.recordId);
                    if (name === undefined)
                        fail('not-found', 'host');
                    m.string(name, { file: 'none', offset: -1 }, 'host');
                    return Object.freeze({ recordId: context.recordId, name });
                });
            }, receive(caller: object, event: ComponentEvent) {
                return attempt(() => {
                    check(caller);
                    if (!context.read || !context.write)
                        fail('denied', 'host');
                    if (!event || typeof event !== 'object' || ![Object.prototype, null].includes(Object.getPrototypeOf(event)))
                        fail('type', 'host');
                    const keys = Reflect.ownKeys(event);
                    if (keys.some(key => typeof key !== 'string') || keys.length !== 2 || !keys.includes('detail') || !keys.includes('type'))
                        fail('type', 'host');
                    const descriptors = Object.getOwnPropertyDescriptors(event);
                    if (!('value' in descriptors.type) || !('value' in descriptors.detail) || descriptors.type.value !== 'save')
                        fail('type', 'host');
                    const name = m.string(descriptors.detail.value, { file: 'none', offset: -1 }, 'host');
                    const tenant = data.get(context.tenant);
                    if (!tenant?.has(context.recordId))
                        fail('not-found', 'host');
                    tenant.set(context.recordId, name);
                });
            }, close() {
                closed = true;
            }
        });
    });
}
