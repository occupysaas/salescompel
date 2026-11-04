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
