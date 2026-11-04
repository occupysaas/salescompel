import { bytes, fail, phaseOf } from './types.ts';
import type { FileTag, Site } from './types.ts';
import type { Meter } from './limits.ts';
export interface Token extends Site {
    kind: 'word' | 'string' | 'symbol' | 'eof';
    text: string;
}
export class Cursor {
    source: string;
    i = 0;
    meter: Meter;
    file: FileTag;
    offsets: number[];
    tokenCount = 0;
    constructor(source: string, file: FileTag, meter: Meter) {
        this.source = source;
        this.file = file;
        this.meter = meter;
        this.offsets = [];
        let n = 0;
        for (let i = 0; i < source.length; i++) {
            this.offsets[i] = n;
            const cp = source.codePointAt(i)!;
            if (cp > 0xffff) {
                this.offsets[++i] = n;
                n += 4;
            }
            else
                n += cp < 128 ? 1 : cp < 2048 ? 2 : 3;
        }
        this.offsets[source.length] = n;
    }
    site(i = this.i): Site {
        return { file: this.file, offset: this.offsets[Math.min(i, this.source.length - 1)] ?? 0 };
    }
    error(code: 'syntax' | 'unsupported' | 'invalid-model' | 'budget', i = this.i): never {
        return fail(code, phaseOf(this.file), this.site(i));
    }
    token(i = this.i) {
        this.meter.check(this.site(i), phaseOf(this.file), 'token');
        if (this.tokenCount >= this.meter.limits.tokens)
            this.error('budget', i);
        this.tokenCount++;
    }
    depth(n: number) {
        if (n > this.meter.limits.nesting)
            this.error('budget');
        this.meter.check(this.site(), phaseOf(this.file), 'depth');
    }
    space() {
        while (/\s/.test(this.source[this.i] ?? '') && this.i < this.source.length)
            this.i++;
    }
    literal(s: string, i = this.i): string {
        return this.meter.string(s, this.site(i), phaseOf(this.file));
    }
}
export class ScriptLexer extends Cursor {
    peeked?: Token;
    structuralDepth = 0;
    peek(): Token {
        if (!this.peeked)
            this.peeked = this.next();
        return this.peeked;
    }
    take(): Token {
        const t = this.peek();
        this.peeked = undefined;
        return t;
    }
    at(text: string): boolean {
        const t = this.peek();
        return t.text === text && t.kind !== 'string';
    }
    accept(text: string): boolean {
        if (this.at(text)) {
            this.take();
            return true;
        }
        return false;
    }
    expect(text: string): Token {
        const t = this.peek();
        if (!this.accept(text))
            this.error('syntax', this.i);
        return t;
    }
    word(): Token {
        const t = this.take();
        if (t.kind !== 'word')
            this.error('syntax');
        return t;
    }
    next(): Token {
        for (;;) {
            this.space();
            if (this.source.startsWith('//', this.i)) {
                while (this.i < this.source.length && this.source[this.i] != '\n')
                    this.i++;
            }
            else
                break;
        }
        const start = this.i;
        const site = this.site();
        if (start >= this.source.length)
            return {
                ...site, kind: 'eof', text: ''
            };
        this.token(start);
        const c = this.source[this.i++];
        if (/[A-Za-z_]/.test(c)) {
            while (/[A-Za-z0-9_]/.test(this.source[this.i] ?? '') && this.i < this.source.length)
                this.i++;
            return {
                ...site, kind: 'word', text: this.literal(this.source.slice(start, this.i), start)
            };
        }
        if (c === "'") {
            let out = '';
            let size = 0;
            while (this.i < this.source.length) {
                let ch = this.source[this.i++];
                if (ch === "'")
                    return {
                        ...site, kind: 'string', text: out
                    };
                if (ch === '\n' || ch === '\r')
                    this.error('syntax', start);
                if (ch === '\\') {
                    ch = this.source[this.i++];
                    if (ch !== "'" && ch !== '\\')
                        this.error('syntax', start);
                }
                else if (ch.charCodeAt(0) >= 0xd800 && ch.charCodeAt(0) <= 0xdbff)
                    ch += this.source[this.i++];
                size += bytes(ch);
                if (size > this.meter.limits.stringBytes)
                    this.error('budget', start);
                out += ch;
            }
            this.error('syntax', start);
        }
        if (!'{}();,.=@:'.includes(c))
            this.error('unsupported', start);
        if (c === '{' || c === '(') {
            this.structuralDepth++;
            this.depth(this.structuralDepth);
        }
        else if (c === '}' || c === ')')
            this.structuralDepth--;
        return {
            ...site, kind: 'symbol', text: c
        };
    }
}
export function entities(text: string, c: Cursor, start: number): string {
    let out = '';
    let total = 0;
    for (let i = 0; i < text.length;) {
        let s: string;
        if (text[i] === '&') {
            const end = text.indexOf(';', i);
            if (end < 0)
                c.error('syntax', start + i);
            const name = text.slice(i + 1, end);
            const literals: Record<string, string> = {
                amp: '&', lt: '<', gt: '>', quot: '"', apos: "'"
            };
            if (Object.hasOwn(literals, name))
                s = literals[name];
            else {
                if (!/^#(?:[0-9]+|x[0-9a-fA-F]+)$/.test(name))
                    c.error('unsupported', start + i);
                const n = name[1] === 'x' ? Number.parseInt(name.slice(2), 16) : Number(name.slice(1));
                if (!Number.isSafeInteger(n) || n <= 0 || n > 0x10ffff || n >= 0xd800 && n <= 0xdfff)
                    c.error('syntax', start + i);
                s = String.fromCodePoint(n);
            }
            i = end + 1;
        }
        else {
            const cp = text.codePointAt(i)!;
            if (cp === 0)
                c.error('syntax', start + i);
            s = String.fromCodePoint(cp);
            i += s.length;
        }
        total += bytes(s);
        if (total > c.meter.limits.stringBytes)
            c.error('budget', start);
        out += s;
    }
    return out;
}
