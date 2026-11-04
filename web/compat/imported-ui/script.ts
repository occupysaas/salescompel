import { ScriptLexer } from './lex.ts';
import type { Meter } from './limits.ts';
import type { Field, Method, Statement, ValueExpr } from './model.ts';
const reserved = new Set(['constructor', 'prototype', '__proto__', 'dispatchEvent', 'LightningElement', 'api', 'render', 'renderedCallback', 'disconnectedCallback', 'errorCallback', 'import', 'export', 'class', 'extends', 'static', 'get', 'set', 'this', 'event']);
export function parseScript(source: string, meter: Meter): {
    fields: Field[];
    methods: Method[];
} {
    const l = new ScriptLexer(source, 'js', meter);
    for (const token of ['import', '{', 'LightningElement', ',', 'api', '}', 'from'])
        l.expect(token);
    const imp = l.take();
    if (imp.kind !== 'string' || imp.text !== 'lwc')
        l.error('unsupported');
    l.expect(';');
    l.expect('export');
    l.expect('default');
    l.expect('class');
    l.word();
    l.expect('extends');
    l.expect('LightningElement');
    l.expect('{');
    l.depth(1);
    const fields: Field[] = [];
    const methods: Method[] = [];
    const names = new Set<string>();
    let statements = 0;
    const value = (): ValueExpr => {
        const t = l.take();
        if (t.kind === 'string')
            return {
                ...t, kind: 'literal', value: t.text
            };
        if (t.kind === 'word' && t.text === 'this') {
            l.expect('.');
            const n = l.word();
            return {
                ...t, kind: 'field', value: n.text
            };
        }
        if (t.kind === 'word' && t.text === 'event') {
            l.expect('.');
            l.expect('target');
            l.expect('.');
            l.expect('value');
            return {
                ...t, kind: 'input', value: ''
            };
        }
        return l.error('unsupported');
    };
    while (!l.at('}')) {
        if (l.peek().kind === 'eof')
            l.error('syntax');
        const pub = l.accept('@');
        if (pub)
            l.expect('api');
        const n = l.word();
        if (names.has(n.text))
            l.error('invalid-model');
        if (reserved.has(n.text))
            l.error('unsupported');
        names.add(n.text);
        if (l.accept('=')) {
            if (fields.length >= meter.limits.fields)
                l.error('budget');
            const v = l.take();
            if (v.kind !== 'string')
                l.error('unsupported');
            l.expect(';');
            fields.push({
                ...n, name: n.text, public: pub, initial: v.text
            });
            continue;
        }
        if (pub)
            l.error('unsupported');
        if (methods.length >= meter.limits.methods)
            l.error('budget');
        l.expect('(');
        const input = l.accept('event');
        l.expect(')');
        l.expect('{');
        l.depth(2);
        const body: Statement[] = [];
        while (!l.at('}')) {
            if (l.peek().kind === 'eof')
                l.error('syntax');
            if (statements >= meter.limits.statements)
                l.error('budget');
            statements++;
            const t = l.expect('this');
            l.expect('.');
            const target = l.word();
            if (target.text === 'dispatchEvent') {
                for (const s of ['(', 'new', 'CustomEvent', '('])
                    l.expect(s);
                const e = l.take();
                if (e.kind !== 'string' || e.text !== 'save')
                    l.error('unsupported');
                l.expect(',');
                l.expect('{');
                l.depth(3);
                l.expect('detail');
                l.expect(':');
                const v = value();
                for (const s of ['}', ')', ')', ';'])
                    l.expect(s);
                body.push({
                    ...t, kind: 'emit', value: v
                });
            }
            else {
                l.expect('=');
                const v = value();
                l.expect(';');
                body.push({
                    ...t, kind: 'assign', target: target.text, value: v
                });
            }
        }
        l.expect('}');
        methods.push({
            ...n, name: n.text, input, statements: body
        });
    }
    l.expect('}');
    if (l.peek().kind !== 'eof')
        l.error('unsupported');
    return { fields, methods };
}
