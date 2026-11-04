import { attempt, Diagnostic, fail, unwrap } from './types.ts';
import type { Limits, Outcome, Phase } from './types.ts';
import { createMeter } from './limits.ts';
import { initialize, prepareAction, prepareProps, copyProps } from './state.ts';
import type { ComponentEvent, ActionPlan } from './state.ts';
import type { Program } from './model.ts';
import { prepareMount, preparePatch } from './renderer.ts';
export interface MountOptions {
    limits: Limits;
    signal?: AbortSignal;
    onEvent?: (event: ComponentEvent) => Outcome<void>;
}
export interface MountedComponent {
    updatePublic(patch: Readonly<Record<string, string>>): Outcome<void>;
    dispose(): void;
    readonly diagnostic: Diagnostic | undefined;
}
const roots = new WeakMap<HTMLDivElement, {
    root: ShadowRoot;
    active: boolean;
}>();
export function mountComponent(container: HTMLDivElement, program: Program, props: Readonly<Record<string, string>>, options: MountOptions): Outcome<MountedComponent> {
    return mountObserved(container, program, props, options);
}
export function mountObserved(container: HTMLDivElement, program: Program, props: Readonly<Record<string, string>>, options: MountOptions, observer?: (label: string) => void): Outcome<MountedComponent> {
    return attempt(() => {
        if (!options)
            fail('invalid-input');
        const limits = { ...options.limits };
        const onEvent = options.onEvent;
        const external = options.signal;
        const local = new AbortController();
        const signal = external ? AbortSignal.any([external, local.signal]) : local.signal;
        const newMeter = () => unwrap(createMeter(limits, signal, observer));
        const m = newMeter();
        const copied = copyProps(props, m);
        if (!container || !container.ownerDocument || container.tagName !== 'DIV' || container.childNodes.length)
            fail('invalid-input');
        const owned = roots.get(container);
        if (owned?.active || container.shadowRoot && container.shadowRoot !== owned?.root || owned?.root.childNodes.length)
            fail('invalid-input');
        if (program.methods.some(f => f.statements.some(s => s.kind === 'emit')) && typeof onEvent !== 'function')
            fail('invalid-input');
        let state = unwrap(initialize(program, copied, m));
        const prepared = prepareMount(program, state, m, container.ownerDocument);
        m.check(program.template, 'mount', 'publish');
        const root = owned?.root ?? container.attachShadow({ mode: 'open' });
        const ownership = { root, active: true };
        roots.set(container, ownership);
        root.append(prepared.fragment);
        let alive = true;
        let diagnostic: Diagnostic | undefined;
        const remove: (() => void)[] = [];
        const action = (planFactory: (meter: ReturnType<typeof newMeter>) => Outcome<ActionPlan>, phase: Phase): Outcome<void> => {
            const out = attempt(() => {
                if (!alive)
                    fail('disposed');
                const meter = newMeter();
                const plan = unwrap(planFactory(meter));
                const publish = preparePatch(prepared.bindings, plan.next, meter, phase);
                meter.check(program.template, phase, 'publish');
                if (!alive)
                    fail('disposed');
                state = plan.next;
                publish();
                for (const event of plan.events) {
                    let result: Outcome<void>;
                    try {
                        result = onEvent!(event);
                    }
                    catch {
                        throw new Diagnostic('host', 'host');
                    }
                    if (!result || typeof result.ok !== 'boolean')
                        fail('host', 'host');
                    if (!result.ok)
                        throw result.error instanceof Diagnostic ? result.error : new Diagnostic('host', 'host');
                }
            });
            diagnostic = out.ok ? undefined : out.error;
            return out;
        };
        for (const e of prepared.events) {
            const listener = () => {
                const value = e.event === 'input' ? (e.node as HTMLInputElement).value : undefined;
                const out = action(m => prepareAction(state, e.method, value, m), 'handler');
                if (!out.ok && alive && e.event === 'input' && e.field)
                    (e.node as HTMLInputElement).value = state.values[e.field];
            };
            e.node.addEventListener(e.event, listener);
            remove.push(() => e.node.removeEventListener(e.event, listener));
        }
        return Object.freeze({
            get diagnostic() {
                return diagnostic;
            }, updatePublic(patch: Readonly<Record<string, string>>) {
                return action(m => prepareProps(state, patch, m), 'update');
            }, dispose() {
                if (!alive)
                    return;
                alive = false;
                local.abort();
                for (const stop of remove)
                    stop();
                root.replaceChildren();
                ownership.active = false;
            }
        });
    });
}
