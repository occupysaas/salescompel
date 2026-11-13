import test from "node:test";
import type { TargetInventory } from "../src/target-inventory-model.ts";
import assert from "node:assert/strict";
import {
  makeInventory,
  makeCompleteInventory,
  makeItem,
  known,
  unknown,
} from "./target-inventory-fixtures.ts";
import { assess } from "./helpers/target-inventory-assess.ts";

import {
  assessTargetInventory,
  repositoryReferences,
} from "../src/target-inventory.ts";
import { createHash } from "node:crypto";
import { stringify } from "yaml";

const semanticCases: [string, (v: TargetInventory) => void, string][] = [
  [
    "complete questions",
    (v) =>
      v.sections.security.questions.push({
        id: "q",
        ownerId: "owner",
        question: "Why?",
        nextAction: "collect",
      }),
    "completion-contradiction",
  ],
  [
    "unknown detail",
    (v) => {
      const i = v.sections.dataMetadata.items[0];
      if (i.category === "object") i.details.name = unknown();
    },
    "completion-contradiction",
  ],
  [
    "unknown required behavior",
    (v) => {
      v.sections.dataMetadata.items[0].behavior = unknown();
    },
    "required-information-missing",
  ],
  [
    "unknown required acceptance",
    (v) => {
      v.sections.dataMetadata.items[0].acceptance = unknown();
    },
    "required-information-missing",
  ],
  [
    "unknown required milestone",
    (v) => {
      v.sections.dataMetadata.items[0].milestone = unknown();
    },
    "required-information-missing",
  ],
  [
    "missing provenance",
    (v) => {
      v.sections.dataMetadata.items[0].provenanceRefs = [];
    },
    "required-information-missing",
  ],
  [
    "missing absence",
    (v) => {
      v.sections.security.absence = null;
    },
    "absence-invalid",
  ],
  [
    "unproven absence",
    (v) => {
      v.sections.security.absence!.evidenceRefs = [];
    },
    "absence-invalid",
  ],
  [
    "nonempty absence",
    (v) => {
      v.sections.dataMetadata.absence = v.sections.security.absence;
    },
    "absence-invalid",
  ],
  [
    "duplicate item",
    (v) => {
      v.sections.dataMetadata.items.push(makeItem());
    },
    "duplicate-id",
  ],
  [
    "duplicate source",
    (v) => {
      v.sources.push({ ...v.sources[0] });
    },
    "duplicate-id",
  ],
  [
    "dangling evidence",
    (v) => {
      v.sections.dataMetadata.items[0].provenanceRefs = ["missing"];
    },
    "dangling-reference",
  ],
  [
    "dangling dependency",
    (v) => {
      v.sections.dataMetadata.items[0].dependencies = ["missing"];
    },
    "dangling-reference",
  ],
  [
    "self dependency",
    (v) => {
      v.sections.dataMetadata.items[0].dependencies = ["account"];
    },
    "dependency-cycle",
  ],
  [
    "unknown disposition",
    (v) => {
      v.sections.dataMetadata.items[0].disposition = "unknown";
    },
    "decision-missing",
  ],
  [
    "empty rationale",
    (v) => {
      v.sections.dataMetadata.items[0].disposition = "out-of-scope";
      v.sections.dataMetadata.items[0].rationale = " ";
    },
    "schema-invalid",
  ],
  [
    "invalid calendar date",
    (v) => {
      v.date = "2026-02-30";
    },
    "date-invalid",
  ],
  [
    "invalid source date",
    (v) => {
      v.sources[0].date = "2025-02-29";
    },
    "date-invalid",
  ],
  [
    "unsafe reference",
    (v) => {
      v.sections.dataMetadata.items[0].repositoryRefs = [
        { path: "docs/compat/../../secret" },
      ];
    },
    "repository-reference-invalid",
  ],
];
for (const [name, mutate, code] of semanticCases)
  test(`assessment rejects ${name}`, () => {
    const v = makeCompleteInventory("real");
    mutate(v);
    const r = assess(v);
    assert.equal(r.validation, "invalid");
    assert.equal(r.readiness, "incomplete");
    assert.ok(
      r.issues.some((i) => i.code === code),
      JSON.stringify(r),
    );
  });
test("cross-item cycles and duplicate question IDs are rejected", () => {
  const v = makeInventory(),
    a = v.sections.dataMetadata.items[0],
    b = makeItem();
  b.id = "contact";
  a.dependencies = ["contact"];
  b.dependencies = ["account"];
  v.sections.dataMetadata.items.push(b);
  assert.ok(assess(v).issues.some((i) => i.code === "dependency-cycle"));
  const q = {
    id: "scope",
    ownerId: "owner",
    question: "What?",
    nextAction: "collect" as const,
  };
  v.sections.security.questions = [q];
  v.sections.localization.questions = [{ ...q }];
  assert.ok(assess(v).issues.some((i) => i.code === "duplicate-id"));
});
test("open questions and unknown optional details produce owned actionable gaps without invalidating partial inventory", () => {
  const v = makeInventory(),
    i = v.sections.dataMetadata.items[0];
  i.disposition = "unknown";
  i.decision = { question: "Required?", nextAction: "review-scope" };
  if (i.category === "object") i.details.name = unknown();
  const r = assess(v);
  assert.equal(r.validation, "valid");
  assert.ok(
    r.gaps.some(
      (g) =>
        g.code === "unknown-disposition" &&
        g.itemId === "account" &&
        g.ownerId === "data-owner" &&
        g.nextAction === "review-scope",
    ),
  );
  assert.ok(
    r.gaps.some(
      (g) => g.code === "unknown-detail" && g.nextAction === "collect",
    ),
  );
});
test("known false and zero do not create unknown-detail gaps", () => {
  const v = makeInventory();
  v.sections.sourceOrg.status = "partial";
  v.sections.sourceOrg.items.push({
    ...makeItem(),
    id: "feature",
    category: "org-feature",
    details: { name: known("Person Accounts"), enabled: known(false) },
  });
  v.sections.dataMetadata.items.push({
    ...makeItem(),
    id: "volume",
    category: "data-volume",
    details: {
      name: known("Accounts"),
      count: known(0),
      unit: known("records"),
    },
  });
  assert.equal(assess(v).validation, "valid");
  assert.equal(
    assess(v).gaps.filter((g) => g.code === "unknown-detail").length,
    0,
  );
});
test("reports bind exact bytes, order deterministically and never include free text or unknown keys", () => {
  const v = makeInventory();
  const marker = "INJECTED_SECRET_1234";
  v.targetLabel = marker;
  v.sources[0].locator = marker;
  v.sources[0].description = marker;
  v.sections.dataMetadata.items[0].rationale = marker;
  v.sections.security.questions = [
    { id: "scope", ownerId: "owner", question: marker, nextAction: "collect" },
  ];
  const bytes = Buffer.from(stringify(v)),
    before = Buffer.from(bytes),
    r = assessTargetInventory(bytes);
  assert.deepEqual(bytes, before);
  assert.equal(r.inputDigest, createHash("sha256").update(bytes).digest("hex"));
  assert.deepEqual(r, assessTargetInventory(bytes));
  assert.ok(!JSON.stringify(r).includes(marker));
  assert.equal(r.gaps[0].section, "sourceOrg");
  const bad: Record<string, any> = structuredClone(v);
  bad[marker] = marker;
  assert.ok(!JSON.stringify(assess(bad)).includes(marker));
  assert.ok(
    !JSON.stringify(
      assessTargetInventory(Buffer.from(`a: [${marker}`)),
    ).includes(marker),
  );
  assert.notEqual(
    assessTargetInventory(Buffer.from(JSON.stringify(v) + " ")).inputDigest,
    r.inputDigest,
  );
});
test("strict parser rejects duplicate keys, documents, alias cycles, oversized bytes and invalid UTF-8", () => {
  const base = JSON.stringify(makeInventory());
  for (const text of [
    "schemaVersion: 1\nschemaVersion: 1",
    '{"x":1,"x":2}',
    "---\na: 1\n---\nb: 2",
    "x: &x [*x]",
    "a: [",
  ])
    assert.equal(
      assessTargetInventory(Buffer.from(text)).validation,
      "invalid",
    );
  assert.equal(
    assessTargetInventory(Buffer.alloc(1048577, 32)).issues[0].code,
    "input-too-large",
  );
  assert.equal(
    assessTargetInventory(Uint8Array.of(0xc3, 0x28)).issues[0].code,
    "encoding-invalid",
  );
  assert.equal(assessTargetInventory(Buffer.from(base)).validation, "valid");
  const deep = "[".repeat(40) + "0" + "]".repeat(40);
  assert.equal(
    assessTargetInventory(Buffer.from(deep)).issues[0].code,
    "parse-invalid",
  );
  const nodes = JSON.stringify(Array.from({ length: 10001 }, () => ({})));
  assert.equal(
    assessTargetInventory(Buffer.from(nodes)).issues[0].code,
    "parse-invalid",
  );
  const bomb = ["x0: &x0 [a, b]"];
  for (let n = 1; n < 10; n++)
    bomb.push(`x${n}: &x${n} [*x${n - 1}, *x${n - 1}]`);
  assert.equal(
    assessTargetInventory(Buffer.from(bomb.join("\n"))).issues[0].code,
    "parse-invalid",
  );
});
test("assessor uses no network and repository references exclude provenance locators", () => {
  const old = globalThis.fetch;
  globalThis.fetch = () => {
    throw new Error("network forbidden");
  };
  try {
    const v = makeInventory();
    v.sources[0].locator = "https://example.invalid/secret";
    v.sections.dataMetadata.items[0].repositoryRefs = [
      { path: "docs/compat/profiles/pilot.yaml", id: "pilot" },
    ];
    assert.equal(assess(v).validation, "valid");
    assert.deepEqual(repositoryReferences(v), [
      { path: "docs/compat/profiles/pilot.yaml", id: "pilot" },
    ]);
  } finally {
    globalThis.fetch = old;
  }
});
