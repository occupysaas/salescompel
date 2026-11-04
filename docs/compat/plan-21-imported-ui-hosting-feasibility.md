# Plan 21 — Imported UI hosting feasibility

Status: execution in progress; no feasibility decision yet.

Base: `b07ebc2b4b851dca98d39065afdc3908f7bf0c0a`, clean main, 2026-11-03.
Node 26.8.1 satisfies the existing >=22.12 floor. Existing dependencies are used;
no engine, SDK, new dependency or third-party implementation is introduced.
A temporary SHA-256 manifest protects all 338 pre-existing tracked files except
this plan's spec/status documents. Plan 19 results and Plan 20's unimplemented
status are independent and unchanged.

[Spec](../superpowers/specs/2026-11-03-plan-21-imported-ui-hosting-feasibility-design.md)
and [implementation plan](../superpowers/plans/2026-11-03-plan-21-imported-ui-hosting-feasibility.md).

## Source ledger

Public official sections accessed 2026-11-03. These are current pages with unknown
applicability to any actual source bundle version; 65.0 is a synthetic input
boundary. No reference-org observations, source examples or implementation code
were collected/copied. Only the indicated vocabulary informs this experiment.

| ID | URL / section | Read boundary and outcome |
| --- | --- | --- |
| html | [HTML file](https://developer.salesforce.com/docs/platform/lwc/guide/create-components-html-file.html) | Opening/root and filename paragraphs read: template-root UI bundle vocabulary. |
| script | [JavaScript file](https://developer.salesforce.com/docs/platform/lwc/guide/create-components-javascript.html) | ES module/default class/LightningElement/fields/handlers paragraphs read. |
| props | [Fields/properties](https://developer.salesforce.com/docs/platform/lwc/guide/js-props-intro.html) | Opening paragraphs read: api marks consumer-facing properties. |
| reactive | [Reactivity](https://developer.salesforce.com/docs/platform/lwc/guide/reactivity-fields.html) | Opening scalar-reactivity paragraphs read; no scheduler/parity inference. |
| metadata | [XML configuration](https://developer.salesforce.com/docs/platform/lwc/guide/reference-configuration-tags.html) | Filename, version and target vocabulary read; current text includes versioned/deprecated constructs outside this subset. |
| events | [Events](https://developer.salesforce.com/docs/platform/lwc/guide/events-create-dispatch.html) | Creation/dispatch and primitive detail paragraphs read. Event timing here is synthetic. |
| dom | [DOM Standard](https://dom.spec.whatwg.org/#dom-document-createtextnode), [shadow attachment](https://dom.spec.whatwg.org/#dom-element-attachshadow) | Text creation and shadow attachment definitions; retrieved living standard dated 2026-10-19. |
| input | [HTML input](https://html.spec.whatwg.org/multipage/input.html#dom-input-value) | Value property and text-control vocabulary; retrieved living standard dated 2026-11-03. Browser behavior is verified separately. |

## Exact independently authored source contract

The spec's allowlists are binding; unsupported content never becomes native code.
Complete source consumption precedes any host read or DOM/listener registration.
UTF-8 byte offsets refer to the original three source strings. ASCII identifiers,
single-quoted JS strings with only quote/backslash escapes, whitespace and JS
line comments are the entire lexical JS subset. All fields have String literal
initializers. All references resolve before execution, including unused methods.

```ebnf
script = "import { LightningElement, api } from", stringLwc, ";",
         "export default class", identifier, "extends LightningElement {",
         {field | method}, "}" ;
field = ["@api"], identifier, "=", string, ";" ;
method = identifier, "(", ["event"], ") {", {statement}, "}" ;
statement = "this.", privateField, "=", value, ";"
          | "this.dispatchEvent(new CustomEvent('save', { detail:", value, "}));" ;
value = string | "this.", declaredField | "event.target.value" ;
```

Here `stringLwc` is exactly 'lwc'. Public assignments and unsupported lifecycle
names are rejected. connectedCallback has no parameters/events; input handlers
have one event parameter, click handlers none. Event-value access is only in a
method referenced by an input binding. No eval/import/Function/ambient objects.

Metadata cardinality: LightningComponentBundle in the exact metadata namespace,
root default xmlns only; optional initial XML declaration; one apiVersion=65.0,
one isExposed=true, one targets containing one target=lightning__RecordPage.
Order is immaterial. Unknown/duplicate children, attributes/namespaces, DTD and
trailing roots fail. No XML/HTML DOM parser is used.

Template: one attribute-free template root. Descendants are section/h2/p/span/
label/input/button; only input is void. Attributes are unique and exactly the
spec whitelist: literal class/title/aria-label; matching type; input value field
and oninput method, button onclick method. Text is literal or {field}; no member
or expression bindings. Decode built-in/numeric entities after recognizing
bindings, so encoded braces stay literal; reject NUL/custom/invalid scalars.
No markup string reaches a live DOM parser. Names and member duplicates follow
the spec's prototype/import/lifecycle exclusions.

Synthetic semantics: private String defaults, exact public props, one connected
callback, staged ordered assignments/one-event queue, prepared DOM patches and
local publication before host delivery. A host denial preserves the successful
local edit and changes no record. Shadow DOM/client policy are not security.
Limits and their counting rules are exactly the spec defaults; source/model/
String/action checks precede retention/publication. Native allocation and total
heap safety are not certified. Browser cancellation cannot preempt a running
synchronous instruction; a late abort cannot undo an already delivered save.

The native Vue/Pinia/router/i18n product is neither imported nor modified. The
synthetic host supplies recordId/name and binds a copied tenant/principal/policy
and record ID; there is no LDS/UI API, real auth, persistence, Flow or Apex bridge.

## Results

Pending implementation, browser execution and fresh whole-change review.
