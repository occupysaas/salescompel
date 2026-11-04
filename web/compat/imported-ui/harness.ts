import type { Scenario } from './test/scenario-data.ts';
import { openRecord, openRecordObserved } from './record-controller.ts';
import { createStore } from './host.ts';
// Trusted developer harness. Fixture JavaScript is imported only as inert raw text.
import html from './testdata/accountCard/accountCard.html?raw';
import js from './testdata/accountCard/accountCard.js?raw';
import metadata from './testdata/accountCard/accountCard.js-meta.xml?raw';
import './harness.css';
import { compileBundle } from './compile.ts';
import { defaultLimits } from './limits.ts';
import { mountComponent, mountObserved } from './component.ts';
import type { MountedComponent } from './component.ts';
import type { SourceBundle, Outcome, Limits } from './types.ts';
import type { ComponentEvent } from './state.ts';
const fixture: SourceBundle = Object.freeze({
    name: 'accountCard', html, js, metadata
});
const events: ComponentEvent[] = [];
const instances: MountedComponent[] = [];
const containers: HTMLDivElement[] = [];
const sourceBundles: SourceBundle[] = [];
function mountInto(container: HTMLDivElement, props = { recordId: '1', name: 'First' }, source = fixture, limits = defaultLimits(), options: {
    cancel?: AbortSignal;
    observer?: (label: string) => void;
} = {}): Outcome<MountedComponent> {
    const p = compileBundle(source, limits, options.cancel);
    if (!p.ok)
        return p;
    return mountObserved(container, p.value, props, {
        limits, signal: options.cancel, onEvent: e => {
            events.push(e);
            return { ok: true, value: undefined };
        }
    }, options.observer);
}
const storeResult = createStore([{
        tenant: 'a', id: '1', name: 'First'
    }, {
        tenant: 'b', id: '1', name: 'Second'
    }, {
        tenant: 'a', id: '2', name: 'Unrelated'
    }]);
if (!storeResult.ok)
    throw storeResult.error;
const store = storeResult.value;
function open(options: {
    tenant?: string;
    recordId?: string;
    read?: boolean;
    write?: boolean;
    source?: SourceBundle;
} = {}) {
    const div = document.createElement('div');
    const identity = { tenant: options.tenant ?? 'a', principal: 'writer' };
    const out = openRecord(div, options.source ?? fixture, {
        identity, policy: {
            identity, readAllowed: options.read ?? true, nameWriteAllowed: options.write ?? true
        }, recordId: options.recordId ?? '1'
    }, store, defaultLimits());
    if (out.ok) {
        document.querySelector('#mounts')!.append(div);
        instances.push(out.value);
        containers.push(div);
        sourceBundles.push(options.source ?? fixture);
    }
    return out;
}
document.querySelector('#mounts')!.addEventListener('click', () => queueMicrotask(() => {
    const error = instances.map(i => i.diagnostic).find(e => e !== undefined);
    document.querySelector('#status')!.textContent = error?.message ?? 'Action completed';
}));
function runScenario(s: Scenario, bundle: SourceBundle) {
    const created = createStore(s.records);
    if (!created.ok)
        throw created.error;
    const store = created.value;
    const div = document.createElement('div');
    const events: string[] = [];
    const out = openRecordObserved(div, bundle, {
        identity: s.identity, policy: s.policy, recordId: s.recordId
    }, store, s.limits, undefined, label => {
        if (label === 'host-save')
            events.push('save');
    });
    if (!out.ok)
        return {
            text: null, input: null, events, store: store.snapshot(), diagnostic: out.error.code
        };
    document.querySelector('#mounts')!.append(div);
    const instance = out.value;
    for (const action of s.actions) {
        if (action.type === 'props')
            instance.updatePublic(action.value);
        else if (action.type === 'click')
            div.shadowRoot!.querySelector('button')!.click();
        else {
            const input = div.shadowRoot!.querySelector('input')!;
            input.value = action.value;
            input.dispatchEvent(new Event('input', { bubbles: true, composed: true }));
        }
    }
    const result = {
        text: div.shadowRoot!.textContent, input: div.shadowRoot!.querySelector('input')?.value ?? null, events, store: store.snapshot(), diagnostic: instance.diagnostic?.code ?? null
    };
    instance.dispose();
    div.remove();
    return result;
}
const probe = {
    runScenario,
    store, open,
    fixture, events, instances, containers, mountInto, mount(props = { recordId: '1', name: 'First' }, source: SourceBundle = fixture, limits: Limits = defaultLimits()) {
        const div = document.createElement('div');
        const out = mountInto(div, props, source, limits);
        if (out.ok) {
            document.querySelector('#mounts')!.append(div);
            instances.push(out.value);
            containers.push(div);
            sourceBundles.push(source);
        }
        return out;
    }, remount(i: number) {
        const out = mountInto(containers[i], { recordId: '1', name: 'First' }, sourceBundles[i]);
        if (out.ok)
            instances[i] = out.value;
        return out;
    }, mountComponent, mountObserved, compileBundle, defaultLimits
};
(window as unknown as {
    probe: typeof probe;
}).probe = probe;
