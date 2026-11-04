import { attempt, fail, unwrap } from './types.ts';
import type { SourceBundle, Limits, Outcome } from './types.ts';
import { createMeter } from './limits.ts';
import { compileObserved } from './compile.ts';
import { bindRecord } from './host.ts';
import type { SyntheticStore, Identity, Policy } from './host.ts';
import { mountComponent } from './component.ts';
import type { MountedComponent } from './component.ts';
export type RecordContext = Readonly<{
    identity: Identity;
    policy: Policy;
    recordId: string;
}>;
export function openRecord(container: HTMLDivElement, bundle: SourceBundle, context: RecordContext, store: SyntheticStore, limits: Limits, signal?: AbortSignal): Outcome<MountedComponent> {
    return openRecordObserved(container, bundle, context, store, limits, signal);
}
export function openRecordObserved(container: HTMLDivElement, bundle: SourceBundle, context: RecordContext, store: SyntheticStore, limits: Limits, signal?: AbortSignal, observer?: (label: string) => void): Outcome<MountedComponent> {
    return attempt(() => {
        const m = unwrap(createMeter(limits, signal));
        if (!context || !context.identity || !context.policy || !context.policy.identity)
            fail('invalid-input');
        const copy = {
            identity: { ...context.identity }, policy: { ...context.policy, identity: { ...context.policy.identity } }, recordId: context.recordId
        };
        const p = unwrap(compileObserved(bundle, m.limits, signal, observer));
        const fields = p.fields.filter(f => f.public).map(f => f.name).sort();
        if (fields.join(',') !== 'name,recordId')
            fail('invalid-model');
        const owner = {};
        const binding = unwrap(bindRecord(store, copy.identity, copy.policy, copy.recordId, owner, m.limits));
        try {
            m.check();
            observer?.('host-read');
            const props = unwrap(binding.load(owner));
            m.check();
            observer?.('mount');
            const component = unwrap(mountComponent(container, p, props, {
                limits: m.limits, signal, onEvent: event => {
                    observer?.('host-save');
                    return binding.receive(owner, event);
                }
            }));
            let closed = false;
            return Object.freeze({
                get diagnostic() {
                    return component.diagnostic;
                }, updatePublic(patch: Readonly<Record<string, string>>) {
                    return attempt(() => {
                        if (closed)
                            fail('disposed');
                        if (!patch || Object.hasOwn(patch, 'recordId'))
                            fail('invalid-input');
                        unwrap(component.updatePublic(patch));
                    });
                }, dispose() {
                    if (closed)
                        return;
                    closed = true;
                    binding.close();
                    component.dispose();
                }
            });
        }
        catch (e) {
            binding.close();
            throw e;
        }
    });
}
