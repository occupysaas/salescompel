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
const probe = {
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
