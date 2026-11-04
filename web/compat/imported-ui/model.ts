import type { Site } from './types.ts';
export interface ValueExpr extends Site {
    kind: 'literal' | 'field' | 'input';
    value: string;
}
export interface Statement extends Site {
    kind: 'assign' | 'emit';
    target?: string;
    value: ValueExpr;
}
export interface Field extends Site {
    name: string;
    public: boolean;
    initial: string;
}
export interface Method extends Site {
    name: string;
    input: boolean;
    statements: readonly Statement[];
}
export interface Attr extends Site {
    name: string;
    kind: 'literal' | 'binding';
    value: string;
}
export interface TemplateNode extends Site {
    kind: 'element' | 'text' | 'binding';
    tag: string;
    text: string;
    children: readonly TemplateNode[];
    attrs: readonly Attr[];
}
export interface Program {
    readonly fields: readonly Field[];
    readonly methods: readonly Method[];
    readonly template: TemplateNode;
}
const compiled = new WeakSet<object>();
export function seal(program: Program): Program { const todo: object[] = [program]; while (todo.length) {
    const o = todo.pop()!;
    for (const v of Object.values(o))
        if (v && typeof v === 'object')
            todo.push(v);
    Object.freeze(o);
} compiled.add(program); return program; }
export function isProgram(p: unknown): p is Program { return typeof p === 'object' && p !== null && compiled.has(p); }
