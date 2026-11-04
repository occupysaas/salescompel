import { test } from 'node:test';
import assert from 'node:assert/strict';
import { compileBundle, compileObserved } from '../compile.ts';
import { defaultLimits } from '../limits.ts';
import { fixture, good, bad } from './helpers.ts';
test('compilePreservedBundle', () => {
    const b = fixture();
    const p = good(compileBundle(b, defaultLimits()));
    assert.equal(p.fields.length, 3);
    assert.equal(p.methods.length, 3);
    assert(Object.isFrozen(p));
    good(compileBundle({
        ...b, name: 'otherCard', js: b.js.replace('AccountCard', 'OtherCard').replaceAll('draft', 'edit'), html: b.html.replaceAll('draft', 'edit')
    }, defaultLimits()));
});
test('rejectWholeBundle', () => {
    const b = fixture();
    const js = [b.js + ' trailing', b.js.replace("from 'lwc'", "from 'other'"), b.js.replace('this.draft = event.target.value;', "this.name = 'x';"), b.js.replace('this.draft = event.target.value;', "this.draft = window.PRIVATE_MARKER;"), b.js.replace(/}\s*$/, "unused() { fetch('PRIVATE_MARKER'); } }"), b.js.replace('handleInput(event)', 'handleInput()'), b.js.replace('draft =', '__proto__ ='), b.js.replace(/}\s*$/, "renderedCallback() {} }"), b.js.replace('this.name;', 'this.missing;'), b.js.replace('draft =', 'name =')];
    for (const s of js)
        bad(compileBundle({ ...b, js: s }, defaultLimits()));
    for (const html of [b.html + '<p>x</p>', b.html.replace('<section>', '<section style="PRIVATE_MARKER">'), b.html.replace('value={draft}', 'value="x"'), b.html.replace('{name}', '{name.x}'), b.html.replace('<h2>', '<script>'), b.html.replace('type="text"', 'type="text" type="text"'), b.html.replace('onclick={handleSave}', 'onclick="PRIVATE_MARKER"'), b.html.replace('handleSave}', 'missing}')])
        bad(compileBundle({
            ...b, js: b.js.replace('event.target.value', 'this.draft'), html
        }, defaultLimits()));
    for (const metadata of [b.metadata + b.metadata, b.metadata.replace('2006', '2007'), b.metadata.replace('<targets>', '<extra>PRIVATE_MARKER</extra><targets>'), b.metadata.replace('65.0', '66.0'), b.metadata.replace('<isExposed>', '<isExposed x="x">'), b.metadata.replace('</targets>', '</targets><isExposed>true</isExposed>')])
        bad(compileBundle({ ...b, metadata }, defaultLimits()));
});
test('decodeLiteralData', () => {
    const b = fixture();
    const p = good(compileBundle({
        ...b, js: b.js.replace('event.target.value', 'this.draft'), html: '<template><p>&#123;name&#125; &amp; &#x1f600;</p></template>'
    }, defaultLimits()));
    assert.equal(p.template.children[0].children[0].text, '{name} & 😀');
    for (const html of ['<template>&bogus;</template>', '<template>&#0;</template>', '<template>&#xD800;</template>', '<template>\ud800</template>'])
        bad(compileBundle({
            ...b, js: b.js.replace('event.target.value', 'this.draft'), html
        }, defaultLimits()));
    good(compileBundle({ ...b, js: b.js.replace("draft = '';", String.raw `draft = 'quote\'slash\\';`) }, defaultLimits()));
});
test('compileBounds', () => {
    const b = fixture();
    for (const k of ['bundleBytes', 'fileBytes', 'tokens', 'nesting', 'fields', 'methods', 'statements', 'templateNodes'] as const)
        bad(compileBundle(b, { ...defaultLimits(), [k]: 1 }), 'budget');
    const size = new TextEncoder().encode(b.js).length;
    good(compileBundle(b, { ...defaultLimits(), fileBytes: size }));
    bad(compileBundle(b, { ...defaultLimits(), fileBytes: size - 1 }), 'budget');
    good(compileBundle(b, {
        ...defaultLimits(), fields: 3, methods: 3, statements: 3
    }));
    bad(compileBundle(b, { ...defaultLimits(), fields: 2 }), 'budget');
    const html = '<template>' + '<section>'.repeat(31) + '</section>'.repeat(31) + '</template>';
    good(compileBundle({
        ...b, js: b.js.replace('event.target.value', 'this.draft'), html
    }, defaultLimits()));
    bad(compileBundle({
        ...b, js: b.js.replace('event.target.value', 'this.draft'), html
    }, { ...defaultLimits(), nesting: 31 }), 'budget');
});
test('compile cancellation is invocation local', () => {
    const c = new AbortController();
    const e = bad(compileObserved(fixture(), defaultLimits(), c.signal, () => c.abort()), 'cancelled');
    assert(e.offset >= 0);
    good(compileBundle(fixture(), defaultLimits()));
});
test('script delimiter nesting counts nested event calls before retention', () => {
    bad(compileBundle(fixture(), { ...defaultLimits(), nesting: 4 }), 'budget');
    good(compileBundle(fixture(), { ...defaultLimits(), nesting: 5 }));
});
test('source is captured before invocation observer can mutate caller bundle', () => {
    const b = { ...fixture() };
    let once = false;
    const p = good(compileObserved(b, defaultLimits(), undefined, () => {
        if (!once) {
            once = true;
            b.html = '<template><p>changed</p></template>';
        }
    }));
    assert.equal(p.template.children[0].tag, 'section');
});
test('all member statement model and bundle limits pass exactly then fail', () => {
    const b = fixture();
    const size = ['html', 'js', 'metadata'].reduce((n, k) => n + new TextEncoder().encode(b[k as 'html' | 'js' | 'metadata']).length, 0);
    good(compileBundle(b, { ...defaultLimits(), bundleBytes: size }));
    bad(compileBundle(b, { ...defaultLimits(), bundleBytes: size - 1 }), 'budget');
    good(compileBundle(b, { ...defaultLimits(), templateNodes: 12 }));
    bad(compileBundle(b, { ...defaultLimits(), templateNodes: 11 }), 'budget');
    for (const [count, key, addition] of [[13, 'fields', (i: number) => `f${i} = '';`], [5, 'methods', (i: number) => `m${i}() {}`], [61, 'statements', (_i: number) => `this.draft = 'x';`]] as const) {
        const extra = (n: number) => key === 'statements' ? `extra() {${Array.from({ length: n }, (_, i) => addition(i)).join('')}}` : Array.from({ length: n }, (_, i) => addition(i)).join('');
        good(compileBundle({ ...b, js: b.js.replace(/}\s*$/, extra(count) + '}') }, defaultLimits()));
        bad(compileBundle({ ...b, js: b.js.replace(/}\s*$/, extra(count + 1) + '}') }, defaultLimits()), 'budget');
    }
});
test('abort during token processing does not produce a program', () => {
    const c = new AbortController();
    let tokens = 0;
    const err = bad(compileObserved(fixture(), defaultLimits(), c.signal, label => {
        if (label === 'token' && ++tokens === 8)
            c.abort();
    }), 'cancelled');
    assert.equal(tokens, 8);
    assert(err.offset > 0);
});
test('ordinary member names outside the explicit reserved set remain supported', () => {
    const b = fixture();
    good(compileBundle({ ...b, js: b.js.replace('draft =', "event = 'unused'; draft =") }, defaultLimits()));
});
test('all JavaScript line terminators end line comments', () => {
    const b = fixture();
    for (const ending of ['\n', '\r', '\r\n', '\u2028', '\u2029']) {
        good(compileBundle({ ...b, js: b.js + '// comment' + ending }, defaultLimits()));
        bad(compileBundle({ ...b, js: b.js + "// comment" + ending + "fetch('PRIVATE_MARKER');" }, defaultLimits()));
    }
});
