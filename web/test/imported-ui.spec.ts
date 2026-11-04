import { scenarios, loadBundle } from '../compat/imported-ui/test/scenario-data.ts';
import { test, expect } from '@playwright/test';
test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForFunction(() => !!(window as any).probe);
});
test('rendersAndEditsPreservedBundle', async ({ page }) => {
    await page.evaluate(() => (window as any).probe.mount({ recordId: '1', name: 'First' }));
    await expect(page.getByRole('heading', { name: 'First', exact: true })).toBeVisible();
    await page.getByRole('textbox', { name: 'Name', exact: true }).fill('Draft');
    await expect(page.getByText('Draft: Draft', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    expect(await page.evaluate(() => (window as any).probe.events)).toEqual([{ type: 'save', detail: 'Draft' }]);
});
test('retainsInputFocusAndCaret', async ({ page }) => {
    await page.evaluate(() => (window as any).probe.mount({ recordId: '1', name: 'First' }));
    const input = page.getByRole('textbox');
    await input.focus();
    await input.evaluate((el: any) => {
        (window as any).inputNode = el;
        el.setSelectionRange(2, 2);
    });
    await page.keyboard.type('xy');
    await expect(input).toHaveValue('Fixyrst');
    expect(await input.evaluate((el: any) => el === (window as any).inputNode && el.getRootNode().activeElement === el && el.selectionStart === 4)).toBe(true);
});
test('rendersMarkupAsText', async ({ page }) => {
    const requests: string[] = [];
    page.on('request', r => requests.push(r.url()));
    const data = '<img src=x onerror=PRIVATE_MARKER><script>PRIVATE_MARKER</script>';
    await page.evaluate(name => (window as any).probe.mount({ recordId: '1', name }), data);
    await expect(page.getByRole('heading', { name: data, exact: true })).toBeVisible();
    expect(await page.locator('#mounts').evaluate(el => [...el.querySelectorAll('div')].some(d => !!d.shadowRoot?.querySelector('img,script')))).toBe(false);
    expect(requests.some(r => r.endsWith('/x'))).toBe(false);
});
test('ownsAndReusesRoot', async ({ page }) => {
    expect(await page.evaluate(() => {
        const p = (window as any).probe;
        p.mount({ recordId: '1', name: 'First' });
        p.mount({ recordId: '2', name: 'Second' });
        const a = p.instances[0];
        const duplicate = p.remount(0);
        a.dispose();
        a.dispose();
        const stale = a.updatePublic({ name: 'stale' });
        const reuse = p.remount(0);
        const foreign = document.createElement('div');
        foreign.attachShadow({ mode: 'open' });
        return {
            duplicate: duplicate.ok, stale: stale.error.code, reuse: reuse.ok, foreign: p.mountInto(foreign).ok
        };
    })).toEqual({
        duplicate: false, stale: 'disposed', reuse: true, foreign: false
    });
    await page.getByRole('textbox').last().fill('Other');
    await expect(page.getByText('Draft: Other', { exact: true })).toBeVisible();
});
test('discardsFailedPatch', async ({ page }) => {
    await page.evaluate(() => (window as any).probe.mount({ recordId: '1', name: 'First' }));
    await page.getByRole('textbox').fill('x'.repeat(4096));
    await expect(page.getByRole('textbox')).toHaveValue('First');
    await expect(page.getByText('Draft: First', { exact: true })).toBeVisible();
    expect(await page.evaluate(() => (window as any).probe.instances[0].diagnostic.code)).toBe('budget');
    expect(await page.evaluate(() => (window as any).probe.events)).toEqual([]);
});
test('loadsAndSavesBoundRecord', async ({ page }) => {
    await page.evaluate(() => (window as any).probe.open({ tenant: 'a' }));
    await page.getByRole('textbox').fill('Changed');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    expect(await page.evaluate(() => (window as any).probe.store.snapshot())).toEqual([{
            tenant: 'a', id: '1', name: 'Changed'
        }, {
            tenant: 'a', id: '2', name: 'Unrelated'
        }, {
            tenant: 'b', id: '1', name: 'Second'
        }]);
});
test('deniedSaveKeepsLocalEdit', async ({ page }) => {
    await page.evaluate(() => (window as any).probe.open({ write: false }));
    await page.getByRole('textbox').fill('Draft');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(page.getByRole('textbox')).toHaveValue('Draft');
    expect(await page.evaluate(() => (window as any).probe.instances[0].diagnostic.code)).toBe('denied');
    expect(await page.evaluate(() => (window as any).probe.store.snapshot()[0].name)).toBe('First');
});
test('isolatesRecordSessions', async ({ page }) => {
    expect(await page.evaluate(() => {
        const p = (window as any).probe;
        const read = p.open({ read: false });
        const missing = p.open({ recordId: 'missing' });
        p.open({ tenant: 'a' });
        p.open({ tenant: 'b' });
        const patch = p.instances[0].updatePublic({ recordId: '2' });
        return {
            read: read.error.code, missing: missing.error.code, count: p.containers.length, patch: patch.error.code
        };
    })).toEqual({
        read: 'denied', missing: 'not-found', count: 2, patch: 'invalid-input'
    });
    await page.getByRole('textbox').last().fill('Tenant B');
    await page.getByRole('button', { name: 'Save', exact: true }).last().click();
    expect(await page.evaluate(() => (window as any).probe.store.snapshot().map((a: any) => a.name))).toEqual(['First', 'Unrelated', 'Tenant B']);
    await page.evaluate(() => (window as any).probe.instances[0].dispose());
    await page.getByRole('textbox').fill('Still B');
    await expect(page.getByText('Draft: Still B', { exact: true })).toBeVisible();
});
test('source rejection leaves browser DOM and host untouched', async ({ page }) => {
    expect(await page.evaluate(() => {
        const p = (window as any).probe;
        const source = { ...p.fixture, js: p.fixture.js + " fetch('PRIVATE_MARKER');" };
        const out = p.open({ source });
        return {
            ok: out.ok, children: document.querySelector('#mounts')!.childElementCount, rows: p.store.snapshot()
        };
    })).toEqual({
        ok: false, children: 0, rows: [{
                tenant: 'a', id: '1', name: 'First'
            }, {
                tenant: 'a', id: '2', name: 'Unrelated'
            }, {
                tenant: 'b', id: '1', name: 'Second'
            }]
    });
});
test('trusted callback exceptions become bounded host diagnostics', async ({ page }) => {
    await page.evaluate(() => {
        const p = (window as any).probe;
        const div = document.createElement('div');
        document.querySelector('#mounts')!.append(div);
        const model = p.compileBundle(p.fixture, p.defaultLimits()).value;
        const out = p.mountComponent(div, model, { recordId: '1', name: 'First' }, { limits: p.defaultLimits(), onEvent: () => {
                throw Error('PRIVATE_MARKER');
            } });
        p.instances.push(out.value);
    });
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    const message = await page.evaluate(() => (window as any).probe.instances[0].diagnostic.message);
    expect(message).toBe('host host none -1');
});
test('cancellation before publication restores input and emits no event', async ({ page }) => {
    await page.evaluate(() => {
        const p = (window as any).probe;
        const div = document.createElement('div');
        document.querySelector('#mounts')!.append(div);
        const c = new AbortController();
        let armed = false;
        const model = p.compileBundle(p.fixture, p.defaultLimits()).value;
        const out = p.mountObserved(div, model, { recordId: '1', name: 'First' }, {
            limits: p.defaultLimits(), signal: c.signal, onEvent: (e: any) => {
                p.events.push(e);
                return { ok: true, value: undefined };
            }
        }, (label: string) => {
            if (armed && label === 'publish')
                c.abort();
        });
        p.instances.push(out.value);
        armed = true;
    });
    await page.getByRole('textbox').fill('Changed');
    await expect(page.getByRole('textbox')).toHaveValue('First');
    await expect(page.getByText('Draft: First', { exact: true })).toBeVisible();
    expect(await page.evaluate(() => (window as any).probe.instances[0].diagnostic.code)).toBe('cancelled');
    expect(await page.evaluate(() => (window as any).probe.events)).toEqual([]);
});
test('step exhaustion after staged assignments never publishes', async ({ page }) => {
    const out = await page.evaluate(() => {
        const p = (window as any).probe;
        const source = { ...p.fixture, js: p.fixture.js.replace("handleSave() {", "handleSave() { " + "this.draft = 'Changed';".repeat(30)) };
        return p.mount({ recordId: '1', name: 'First' }, source, { ...p.defaultLimits(), steps: 50 }).ok;
    });
    expect(out).toBe(true);
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(page.getByRole('textbox')).toHaveValue('First');
    expect(await page.evaluate(() => (window as any).probe.instances[0].diagnostic.code)).toBe('budget');
    expect(await page.evaluate(() => (window as any).probe.events)).toEqual([]);
});
test('one compiled model serves independent live instances', async ({ page }) => {
    await page.evaluate(() => {
        const p = (window as any).probe;
        const model = p.compileBundle(p.fixture, p.defaultLimits()).value;
        for (const name of ['First', 'Second']) {
            const div = document.createElement('div');
            document.querySelector('#mounts')!.append(div);
            const out = p.mountComponent(div, model, { recordId: name, name }, { limits: p.defaultLimits(), onEvent: (e: any) => {
                    p.events.push(e);
                    return { ok: true, value: undefined };
                } });
            p.instances.push(out.value);
        }
    });
    await page.getByRole('textbox').first().fill('Edit A');
    await page.evaluate(() => (window as any).probe.instances[0].updatePublic({ name: 'Refreshed' }));
    await expect(page.getByRole('heading', { name: 'Refreshed', exact: true })).toBeVisible();
    await expect(page.getByRole('textbox').first()).toHaveValue('Edit A');
    await expect(page.getByRole('textbox').last()).toHaveValue('Second');
    await page.evaluate(() => (window as any).probe.instances[0].dispose());
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    expect(await page.evaluate(() => (window as any).probe.events)).toEqual([{ type: 'save', detail: 'Second' }]);
});
// JSON expectations are independent inputs; every browser assertion runs on live DOM.
for (const scenario of scenarios)
    test('browser scenario ' + scenario.id, async ({ page }) => {
        const bundle = loadBundle(scenario.bundle);
        for (let repeat = 0; repeat < 2; repeat++) {
            if (repeat)
                await page.goto('/');
            await page.waitForFunction(() => !!(window as any).probe?.runScenario);
            const result = await page.evaluate(({ scenario, bundle }) => (window as any).probe.runScenario(scenario, bundle), { scenario, bundle });
            expect(result).toEqual(scenario.expected);
        }
    });
test('stale DOM listeners cannot save after disposal', async ({ page }) => {
    await page.evaluate(() => (window as any).probe.open());
    await page.getByRole('textbox').fill('Changed');
    expect(await page.evaluate(() => {
        const p = (window as any).probe;
        const button = p.containers[0].shadowRoot.querySelector('button');
        p.instances[0].dispose();
        button.click();
        return p.store.snapshot()[0].name;
    })).toBe('First');
});
test('raw fixture text is preserved by the harness loader', async ({ page }) => {
    expect(await page.evaluate(() => (window as any).probe.fixture)).toEqual(loadBundle('accountCard'));
});
test('failed input handlers restore unbound controls to their last accepted value', async ({ page }) => {
    await page.evaluate(() => {
        const p = (window as any).probe;
        const source = { ...p.fixture, html: p.fixture.html.replace('value={draft}', '') };
        const program = p.compileBundle(source, p.defaultLimits()).value;
        const div = document.createElement('div');
        document.querySelector('#mounts')!.append(div);
        const cancel = new AbortController();
        let armed = false;
        const mounted = p.mountObserved(div, program, { recordId: '1', name: 'First' }, {
            limits: p.defaultLimits(), signal: cancel.signal,
            onEvent: (event: any) => {
                p.events.push(event);
                return { ok: true, value: undefined };
            }
        }, (label: string) => {
            if (armed && label === 'publish')
                cancel.abort();
        });
        p.instances.push(mounted.value);
        p.arm = () => {
            armed = true;
        };
    });
    const input = page.getByRole('textbox');
    await expect(input).toHaveValue('');
    await page.evaluate(() => (window as any).probe.arm());
    await input.fill('Changed');
    await expect(input).toHaveValue('');
    await expect(page.getByText('Draft: First', { exact: true })).toBeVisible();
    expect(await page.evaluate(() => (window as any).probe.instances[0].diagnostic.code)).toBe('cancelled');
    expect(await page.evaluate(() => (window as any).probe.events)).toEqual([]);
});
test('later failed unbound edits preserve earlier accepted input', async ({ page }) => {
    await page.evaluate(() => {
        const p = (window as any).probe;
        const source = { ...p.fixture, html: p.fixture.html.replace('value={draft}', '') };
        p.mount({ recordId: '1', name: 'First' }, source);
    });
    const input = page.getByRole('textbox');
    await input.fill('Accepted');
    await expect(page.getByText('Draft: Accepted', { exact: true })).toBeVisible();
    await input.fill('x'.repeat(4096));
    await expect(input).toHaveValue('Accepted');
    await expect(page.getByText('Draft: Accepted', { exact: true })).toBeVisible();
    expect(await page.evaluate(() => (window as any).probe.instances[0].diagnostic.code)).toBe('budget');
});
test('foreign closed roots reject with a diagnostic and retain their content', async ({ page }) => {
    const result = await page.evaluate(() => {
        const p = (window as any).probe;
        const div = document.createElement('div');
        const root = div.attachShadow({ mode: 'closed' });
        const content = document.createTextNode('foreign');
        root.append(content);
        const mounted = p.mountInto(div);
        return {
            ok: mounted.ok, code: mounted.error?.code, same: root.firstChild === content, text: root.textContent, count: root.childNodes.length
        };
    });
    expect(result).toEqual({
        ok: false, code: 'invalid-input', same: true, text: 'foreign', count: 1
    });
});
