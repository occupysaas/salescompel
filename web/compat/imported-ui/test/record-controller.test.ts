import { test } from 'node:test';
import assert from 'node:assert/strict';
import { openRecordObserved } from '../record-controller.ts';
import { createStore } from '../host.ts';
import { defaultLimits } from '../limits.ts';
import { fixture, good, bad } from './helpers.ts';
test('rejectsBeforeReadOrDOM', () => {
    const b = fixture();
    for (const source of [{ ...b, js: b.js + ' PRIVATE_MARKER' }, { ...b, html: b.html + '<script>PRIVATE_MARKER</script>' }, { ...b, metadata: b.metadata.replace('65.0', '99.0') }, { ...b, js: b.js.replace(/}\s*$/, "unused() { fetch('PRIVATE_MARKER'); } }") }]) {
        const store = good(createStore([{
                tenant: 'a', id: '1', name: 'First'
            }]));
        const before = store.snapshot();
        const events: string[] = [];
        const identity = { tenant: 'a', principal: 'writer' };
        bad(openRecordObserved({} as HTMLDivElement, source, {
            identity, policy: {
                identity, readAllowed: true, nameWriteAllowed: true
            }, recordId: '1'
        }, store, defaultLimits(), undefined, label => events.push(label)));
        assert(!events.includes('host-read'));
        assert(!events.includes('mount'));
        assert.deepEqual(store.snapshot(), before);
    }
});
