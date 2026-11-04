export type SourceBundle = Readonly<{
    name: string;
    html: string;
    js: string;
    metadata: string;
}>;
export interface Limits {
    bundleBytes: number;
    fileBytes: number;
    tokens: number;
    nesting: number;
    fields: number;
    methods: number;
    statements: number;
    templateNodes: number;
    stringBytes: number;
    steps: number;
    events: number;
}
export type FileTag = 'html' | 'js' | 'metadata' | 'none';
export type Phase = 'input' | 'metadata' | 'script' | 'template' | 'mount' | 'update' | 'handler' | 'host';
export type Code = 'invalid-input' | 'syntax' | 'unsupported' | 'invalid-model' | 'type' | 'denied' | 'not-found' | 'budget' | 'cancelled' | 'disposed' | 'host';
export interface Site {
    file: FileTag;
    offset: number;
}
export class Diagnostic extends Error {
    readonly code: Code;
    readonly phase: Phase;
    readonly file: FileTag;
    readonly offset: number;
    constructor(code: Code, phase: Phase = 'input', file: FileTag = 'none', offset = -1) { super(`${code} ${phase} ${file} ${offset}`); this.name = 'Diagnostic'; this.code = code; this.phase = phase; this.file = file; this.offset = offset; }
}
export type Outcome<T> = {
    ok: true;
    value: T;
} | {
    ok: false;
    error: Diagnostic;
};
export function attempt<T>(f: () => T): Outcome<T> { try {
    return { ok: true, value: f() };
}
catch (e) {
    if (e instanceof Diagnostic)
        return { ok: false, error: e };
    throw e;
} }
export function unwrap<T>(o: Outcome<T>): T { if (!o.ok)
    throw o.error; return o.value; }
export function fail(code: Code, phase: Phase = 'input', site: Site = { file: 'none', offset: -1 }): never { throw new Diagnostic(code, phase, site.file, site.offset); }
export function bytes(s: string): number { let n = 0; for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    if (c >= 0xd800 && c <= 0xdbff) {
        const d = s.charCodeAt(++i);
        if (!(d >= 0xdc00 && d <= 0xdfff))
            throw new Diagnostic('type');
        n += 4;
    }
    else if (c >= 0xdc00 && c <= 0xdfff)
        throw new Diagnostic('type');
    else
        n += c < 128 ? 1 : c < 2048 ? 2 : 3;
} return n; }
export const phaseOf = (file: FileTag): Phase => file === 'js' ? 'script' : file === 'html' ? 'template' : file === 'metadata' ? 'metadata' : 'input';
