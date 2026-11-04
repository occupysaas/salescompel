import { attempt, bytes, fail } from './types.ts';
import type { Limits, Outcome, Site, Phase } from './types.ts';
export function defaultLimits(): Limits { return { bundleBytes: 49152, fileBytes: 16384, tokens: 4096, nesting: 32, fields: 16, methods: 8, statements: 64, templateNodes: 128, stringBytes: 4096, steps: 512, events: 1 }; }
export class Meter {
    readonly limits: Readonly<Limits>;
    readonly signal?: AbortSignal;
    readonly observer?: (label: string) => void;
    used: Record<string, number> = Object.create(null);
    constructor(l: Limits, signal?: AbortSignal, observer?: (label: string) => void) { for (const k of Object.keys(l))
        this.used[k] = 0; this.limits = Object.freeze({ ...l }); this.signal = signal; this.observer = observer; }
    check(site: Site = { file: 'none', offset: -1 }, phase: Phase = 'input', label = 'check') { if (this.signal?.aborted)
        fail('cancelled', phase, site); this.observer?.(label); if (this.signal?.aborted)
        fail('cancelled', phase, site); }
    charge(key: keyof Limits, n = 1, site: Site = { file: 'none', offset: -1 }, phase: Phase = 'input') { this.check(site, phase, key); if (!Number.isSafeInteger(n) || n < 0)
        fail('invalid-input'); const used = this.used[key] ?? 0; if (n > this.limits[key] - used)
        fail('budget', phase, site); this.used[key] = used + n; }
    step(site: Site = { file: 'none', offset: -1 }, phase: Phase = 'input') { this.charge('steps', 1, site, phase); }
    string(s: unknown, site: Site = { file: 'none', offset: -1 }, phase: Phase = 'input'): string { if (typeof s !== 'string')
        fail('type', phase, site); let n: number; try {
        n = bytes(s);
    }
    catch {
        fail('type', phase, site);
    } if (n > this.limits.stringBytes)
        fail('budget', phase, site); return s; }
}
export function createMeter(l: Limits, signal?: AbortSignal, observer?: (label: string) => void): Outcome<Meter> { return attempt(() => { const keys = Object.keys(defaultLimits()); if (!l || keys.some(k => !Number.isSafeInteger(l[k as keyof Limits]) || l[k as keyof Limits] <= 0) || l.events !== 1)
    fail('invalid-input'); return new Meter(l, signal, observer); }); }
