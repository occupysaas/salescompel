import { test } from 'node:test';
import assert from 'node:assert/strict';
import { defaultLimits, createMeter } from '../limits.ts';
import { good, bad } from './helpers.ts';
test('meterBounds', () => {
    for (const key of Object.keys(defaultLimits()))
        for (const value of [0, -1, 0.5, NaN, Infinity, Number.MAX_SAFE_INTEGER + 1])
            bad(createMeter({ ...defaultLimits(), [key]: value }), 'invalid-input');
    bad(createMeter({ ...defaultLimits(), events: 2 }), 'invalid-input');
    const m = good(createMeter({ ...defaultLimits(), steps: 2 }));
    m.step();
    m.step();
    assert.throws(() => m.step(), { code: 'budget' });
    const other = good(createMeter(defaultLimits()));
    other.step();
    assert.throws(() => other.charge('steps', Number.MAX_SAFE_INTEGER), { code: 'budget' });
    assert.throws(() => other.charge('steps', -1), { code: 'invalid-input' });
    const c = new AbortController();
    c.abort();
    const cancelled = good(createMeter(defaultLimits(), c.signal));
    assert.throws(() => cancelled.step(), { code: 'cancelled' });
    assert.equal(cancelled.used.steps, 0);
});
test('default counters and decoded UTF8 boundaries are exact', async () => {
    const { ScriptLexer } = await import('../lex.ts');
    const { parseTemplate } = await import('../markup.ts');
    const text = "'" + 'é'.repeat(2048) + "'";
    assert.equal(new ScriptLexer(text, 'js', good(createMeter(defaultLimits()))).take().text.length, 2048);
    assert.throws(() => new ScriptLexer(text.slice(0, -1) + "é'", 'js', good(createMeter(defaultLimits()))).take(), { code: 'budget' });
    const one = new ScriptLexer("'x'", 'js', good(createMeter({ ...defaultLimits(), tokens: 1 })));
    one.take();
    assert.equal(one.take().kind, 'eof');
    const two = new ScriptLexer("'x' 'y'", 'js', good(createMeter({ ...defaultLimits(), tokens: 1 })));
    two.take();
    assert.throws(() => two.take(), { code: 'budget' });
    const html = '<template>' + '<span></span>'.repeat(584) + 'x</template>';
    parseTemplate(html, good(createMeter({ ...defaultLimits(), templateNodes: 1000 })));
    assert.throws(() => parseTemplate(html, good(createMeter({
        ...defaultLimits(), templateNodes: 1000, tokens: 4095
    }))), { code: 'budget' });
});
