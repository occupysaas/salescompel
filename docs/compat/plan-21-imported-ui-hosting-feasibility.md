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

The isolated implementation and initial verification ran on 2026-11-03.
Current result: 43 experiment Node tests and 37 dedicated Chromium tests passed,
with no skips. The JSON matrix has 22 scenarios; each runs twice in both the
Node model/host runner and real browser runner. Actual outcomes are recorded by
the test runners, not inferred from expected JSON. Fresh whole-change review and
final native-product verification are pending.

Reproduction (from `web/`):

```sh
node --experimental-strip-types --test compat/imported-ui/test/*.test.ts
node_modules/.bin/tsc -p compat/imported-ui/tsconfig.json --noEmit
CHROMIUM_PATH=/usr/bin/chromium node_modules/.bin/playwright test -c playwright.imported-ui.config.ts
node_modules/.bin/vite build --config compat/imported-ui/vite.config.ts
```

| Required experiment | Actual tests / assertions | Result |
| --- | --- | --- |
| Unchanged source → DOM | `compilePreservedBundle`, JSON load/edit-save/tenant-b, `rendersAndEditsPreservedBundle`, raw loader equality | Passed: original per-file SHA-256 remains identical across inputs/repeats; actual keyboard/input/click updates draft, emits exact String detail and changes only bound Name. |
| Public/private lifecycle | `initializePrivateState`, `prepareOrderedAction`, public-update scenario, compiled-model browser reuse | Passed: one connected callback, private edits retained across public updates, no shared model/state/listeners. |
| Browser usability | `retainsInputFocusAndCaret`, `rendersMarkupAsText`, root/disposal tests | Passed: actual Chromium node identity/focus/caret retained, accessible named controls, markup-shaped text inert, other instance remains usable. |
| Host boundary | `recordBoundReadAndSave`, `rejectHostCapabilities`, source-first integration/browser session tests | Passed: same ID across tenants, complete-store equality on denied/missing/wrong-owner/stale/malformed events, fixed captured record ID. Denied save keeps draft and preserves all records. |
| Complete rejection | `rejectWholeBundle`, `rejectsBeforeReadOrDOM`, unsupported JSON bundles, browser DOM absence | Passed: connected/unused/trailing bad source rejects before host read or component DOM/listeners. No fixture-name dispatch. |
| Bounds/failure atomicity | compile/meter exact boundaries, UTF-8/entity counters, event/steps cases, `discardsFailedPatch` | Passed: configured byte/token/depth/member/statement/node/String/step/event checks; failed staged edits restore prior model/DOM/native input and emit nothing. |
| Cancellation/disposal | token abort, `abortPreparedAction`, browser cancellation before publication, reused/disposed roots, stale DOM click | Passed: deterministic private checkpoints, no sleeps, no late store write from disposed component. |
| Repeatability/isolation | all 22 JSON cases ×2 per runner, compiled-model browser reuse | Passed: model/host assertions are Node measurements; DOM/focus/listener behavior is measured in Chromium, never claimed from Node tests. |

All fixtures are independently authored and synthetic. Dedicated Node tests also
check malformed scenario schemas and safe fixture basenames. Error messages expose
only category/phase/file/offset; injected source/data markers are never returned.
The controller's private observer labels include host-read, mount and host-save,
without values. This is instrumentation for the experiment, not a production API.

During implementation, regressions exposed and corrected three boundary details:
script nesting must count nested event-call delimiters; source strings must be
captured before observer callbacks can mutate the caller bundle; ordinary member
names outside the explicitly reserved set remain legal. Tests failed first and
passed after correction. Unknown general calls are classified unsupported rather
than mistaken for malformed supported statements.

## Architecture decision and follow-up gates

Provisional decision pending review/final checks: **Continue** for this tiny
independently interpreted subset only. Source-driven rendering and fixed record
bindings work without hand-translating fixtures into Vue. The interpreter exposes
no JavaScript evaluator or ambient APIs, and safe DOM sinks are inspectable.
This supplies evidence for a next bounded slice; it does not answer full-JavaScript
execution or real-target LWC hosting. The synthetic event queue is deliberately
published after local state/DOM, and host denial has different rollback scope.

An eligible existing LWC runtime would require an explicit license/provenance
decision plus platform-service and version evidence. An isolated full-JavaScript
host would require a process/origin/capability design and real resource/security
verification. Neither alternative was installed, implemented or benchmarked;
passing this interpreter does not establish either alternative's feasibility.

| Owner | Gate / next action |
| --- | --- |
| maintainer | Select a real target and assign imported-component inventory owner; collect exact bundle/API versions, required modules/components, locales and permitted source. |
| platform-implementer | Design evidence-backed expansion for full JS, dependency resolution, reactivity/event/lifecycle ordering, real services and protected execution. No current DOM/client-policy security claim. |
| maintainer | Decide eligibility/provenance before any future runtime/SDK reuse; this package adds no such dependency. |
| platform-implementer | Separate studies for Aura/Visualforce, base components/wire/LDS/UI API, Apex/Flow, styles/navigation/packages and full locale/accessibility fidelity. |

No reference org, database or live client was used. Native Vue/i18n/product routes,
Plan 19 results, Plan 20 implementation status, Plans 10/14 and all milestone gates
and estimates remain unchanged. Native DOM allocation, aggregate heap protection,
synchronous-instruction preemption and rollback after a delivered host save are
not certified by this experiment.
