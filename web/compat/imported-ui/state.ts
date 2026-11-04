import { attempt, fail } from './types.ts';
import type { Outcome } from './types.ts';
import type { Meter } from './limits.ts';
import { isProgram } from './model.ts';
import type { Program, Method, ValueExpr } from './model.ts';
export interface ComponentEvent {
    readonly type: 'save';
    readonly detail: string;
}
export interface InstanceState {
    readonly program: Program;
    readonly values: Readonly<Record<string, string>>;
}
export interface ActionPlan {
    readonly next: InstanceState;
    readonly events: readonly ComponentEvent[];
}
export function copyProps(raw: Readonly<Record<string, string>>, m: Meter): Readonly<Record<string, string>> {
    if (!raw || typeof raw !== 'object' || ![Object.prototype, null].includes(Object.getPrototypeOf(raw)))
        fail('invalid-input');
    const keys = Reflect.ownKeys(raw);
    if (keys.length > m.limits.fields)
        fail('budget');
    const result: Record<string, string> = Object.create(null);
    for (const k of keys) {
        if (typeof k !== 'string')
            fail('invalid-input');
        const descriptor = Object.getOwnPropertyDescriptor(raw, k)!;
        if (!('value' in descriptor))
            fail('invalid-input');
        result[k] = m.string(descriptor.value);
    }
    return Object.freeze(result);
}
function state(program: Program, values: Record<string, string>): InstanceState {
    return Object.freeze({ program, values: Object.freeze(values) });
}
function perform(s: InstanceState, method: Method, input: string | undefined, m: Meter): ActionPlan {
    m.step(method, 'handler');
    if (method.input)
        m.string(input, method, 'handler');
    else if (input !== undefined)
        fail('type');
    const values = { ...s.values };
    const events: ComponentEvent[] = [];
    const resolve = (v: ValueExpr): string => {
        m.step(v, 'handler');
        return m.string(v.kind === 'literal' ? v.value : v.kind === 'input' ? input : values[v.value], v, 'handler');
    };
    for (const statement of method.statements) {
        m.step(statement, 'handler');
        const v = resolve(statement.value);
        if (statement.kind === 'assign')
            values[statement.target!] = v;
        else {
            m.charge('events', 1, statement, 'handler');
            events.push(Object.freeze({ type: 'save', detail: v }));
        }
    }
    return Object.freeze({ next: state(s.program, values), events: Object.freeze(events) });
}
export function initialize(program: Program, props: Readonly<Record<string, string>>, m: Meter): Outcome<InstanceState> {
    return attempt(() => {
        if (!isProgram(program))
            fail('invalid-input');
        const copy = copyProps(props, m);
        const publics = program.fields.filter(f => f.public);
        if (Object.keys(copy).length !== publics.length || Object.keys(copy).some(k => !publics.some(f => f.name === k)))
            fail('invalid-input');
        m.check(program.template, 'mount', 'initialize');
        const values: Record<string, string> = Object.create(null);
        for (const f of program.fields) {
            m.step(f, 'mount');
            values[f.name] = m.string(f.public ? copy[f.name] : f.initial, f, 'mount');
        }
        const initial = state(program, values);
        const connected = program.methods.find(f => f.name === 'connectedCallback');
        return connected ? perform(initial, connected, undefined, m).next : initial;
    });
}
export function prepareAction(s: InstanceState, name: string, input: string | undefined, m: Meter): Outcome<ActionPlan> {
    return attempt(() => {
        const method = s.program.methods.find(f => f.name === name);
        if (!method || method.name === 'connectedCallback')
            fail('invalid-input');
        return perform(s, method, input, m);
    });
}
export function prepareProps(s: InstanceState, patch: Readonly<Record<string, string>>, m: Meter): Outcome<ActionPlan> {
    return attempt(() => {
        const copy = copyProps(patch, m);
        for (const key of Object.keys(copy))
            if (!s.program.fields.some(f => f.public && f.name === key))
                fail('invalid-input');
        const values = { ...s.values };
        for (const [k, v] of Object.entries(copy)) {
            const field = s.program.fields.find(f => f.name === k)!;
            m.step(field, 'update');
            values[k] = v;
        }
        return Object.freeze({ next: state(s.program, values), events: Object.freeze([]) });
    });
}
