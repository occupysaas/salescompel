import { fail, bytes } from './types.ts';
import type { Phase, Site } from './types.ts';
import type { Meter } from './limits.ts';
import type { Program, TemplateNode } from './model.ts';
import type { InstanceState } from './state.ts';
export type Binding = {
    kind: 'text';
    node: Text;
    parts: readonly TemplateNode[];
    site: Site;
} | {
    kind: 'input';
    node: HTMLInputElement;
    field: string;
    site: Site;
};
export interface EventBinding {
    node: HTMLElement;
    event: 'input' | 'click';
    method: string;
    field?: string;
}
export interface PreparedRender {
    fragment: DocumentFragment;
    bindings: Binding[];
    events: EventBinding[];
}
function text(parts: readonly TemplateNode[], state: InstanceState, m: Meter, phase: Phase): string {
    let value = '';
    let size = 0;
    for (const p of parts) {
        m.step(p, phase);
        const v = m.string(p.kind === 'binding' ? state.values[p.text] : p.text, p, phase);
        const n = bytes(v);
        if (n > m.limits.stringBytes - size)
            fail('budget', phase, p);
        size += n;
        value += v;
    }
    return value;
}
export function prepareMount(program: Program, state: InstanceState, m: Meter, doc: Document): PreparedRender {
    const fragment = doc.createDocumentFragment();
    const bindings: Binding[] = [];
    const events: EventBinding[] = [];
    const children = (nodes: readonly TemplateNode[], parent: Node) => {
        for (let i = 0; i < nodes.length; i++) {
            const n = nodes[i];
            m.step(n, 'mount');
            if (n.kind !== 'element') {
                const parts: TemplateNode[] = [n];
                while (i + 1 < nodes.length && nodes[i + 1].kind !== 'element')
                    parts.push(nodes[++i]);
                const value = text(parts, state, m, 'mount');
                const node = doc.createTextNode(value);
                parent.appendChild(node);
                bindings.push({
                    kind: 'text', node, parts, site: n
                });
                continue;
            }
            const el = doc.createElement(n.tag);
            for (const a of n.attrs) {
                m.step(a, 'mount');
                if (a.kind === 'literal')
                    el.setAttribute(a.name, a.value);
                else if (a.name === 'value') {
                    const v = m.string(state.values[a.value], a, 'mount');
                    (el as HTMLInputElement).value = v;
                    bindings.push({
                        kind: 'input', node: el as HTMLInputElement, field: a.value, site: a
                    });
                }
                else
                    events.push({
                        node: el, event: a.name === 'oninput' ? 'input' : 'click', method: a.value, field: n.attrs.find(a => a.name === 'value')?.value
                    });
            }
            children(n.children, el);
            parent.appendChild(el);
        }
    };
    children(program.template.children, fragment);
    return {
        fragment, bindings, events
    };
}
export function preparePatch(bindings: readonly Binding[], state: InstanceState, m: Meter, phase: Phase): () => void {
    const patches: {
        b: Binding;
        value: string;
    }[] = [];
    for (const b of bindings) {
        m.step(b.site, phase);
        const value = b.kind === 'text' ? text(b.parts, state, m, phase) : m.string(state.values[b.field], b.site, phase);
        patches.push({ b, value });
    }
    return () => {
        for (const { b, value } of patches) {
            if (b.kind === 'text') {
                if (b.node.data !== value)
                    b.node.data = value;
            }
            else if (b.node.value !== value)
                b.node.value = value;
        }
    };
}
