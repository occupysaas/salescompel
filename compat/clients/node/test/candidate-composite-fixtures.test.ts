import assert from "node:assert/strict";
import test from "node:test";
import { mkdtemp, writeFile, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { runCompositeFixtureAttempt } from "../src/candidate-composite-fixtures.ts";
import {
  cleanupFixtures,
  recordFixtureEvent,
  type FixtureEvent,
  type Response,
} from "../src/candidate-acceptance-fixtures.ts";
import type { GateContext } from "../src/candidate-acceptance-preflight.ts";
const runId = "a".repeat(24);
const aid = "001000000000000AAA",
  cid = "003000000000000AAA";
const intents: FixtureEvent[] = [
  {
    schemaVersion: 1,
    runId,
    operationId: "a",
    object: "Account",
    principal: "admin",
    names: ["Synthetic"],
    kind: "intent",
  },
  {
    schemaVersion: 1,
    runId,
    operationId: "c",
    object: "Contact",
    principal: "admin",
    names: ["Synthetic"],
    kind: "intent",
  },
];
const expected = [
  { referenceId: "a", method: "POST" },
  { referenceId: "c", method: "POST" },
] as const;
const success = (referenceId: string, id: string) => ({
  referenceId,
  httpStatusCode: 201,
  httpHeaders: {},
  body: { id, success: true, errors: [] },
});
const rollback = (referenceId: string, code = "PROCESSING_HALTED") => ({
  referenceId,
  httpStatusCode: 400,
  httpHeaders: {},
  body: [{ errorCode: code, message: "Rollback" }],
});
async function setup(t: any) {
  const dir = await mkdtemp(join(tmpdir(), "p24-fixture-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const path = join(dir, "journal");
  await writeFile(path, "", { mode: 0o600 });
  return path;
}
const events = async (path: string) =>
  (await readFile(path, "utf8"))
    .trim()
    .split("\n")
    .filter(Boolean)
    .map((x) => JSON.parse(x) as FixtureEvent);
const context = (path: string) =>
  ({ fixtureJournal: path, runId }) as GateContext;
test("Composite journals every intent before dispatch and attributable IDs before assertions", async (t) => {
  const path = await setup(t);
  let called = false;
  const response = await runCompositeFixtureAttempt(
    path,
    intents,
    expected,
    "commit",
    async () => {
      assert.deepEqual(
        (await events(path)).map((x) => x.kind),
        ["intent", "intent"],
      );
      called = true;
      return {
        status: 200,
        body: { compositeResponse: [success("a", aid), success("c", cid)] },
      };
    },
    async () => {
      throw Error("unexpected");
    },
  );
  assert.ok(called);
  assert.equal(response.status, 200);
  assert.deepEqual(
    (await events(path)).filter((x) => x.kind === "known").map((x) => x.id),
    [aid, cid],
  );
  const deleted: string[] = [];
  const alive = new Set([aid, cid]);
  const cleanup = await cleanupFixtures(
    context(path),
    async (_c, _r, url, method) => {
      const id = url.split("/").at(-1)!;
      if (method === "DELETE") {
        deleted.push(id);
        alive.delete(id);
        return { status: 204, body: null };
      }
      return alive.has(id)
        ? { status: 200, body: { Name: "Synthetic", LastName: "Synthetic" } }
        : { status: 404, body: null };
    },
  );
  assert.equal(cleanup.verified, true);
  assert.deepEqual(deleted, [cid, aid]);
});
test("Composite does not dispatch after a journal append failure", async (t) => {
  const path = await setup(t);
  let called = false;
  await assert.rejects(
    runCompositeFixtureAttempt(
      path,
      [intents[0], { ...intents[1], names: [] }],
      expected,
      "commit",
      async () => {
        called = true;
        return { status: 200, body: {} };
      },
      async () => {},
    ),
  );
  assert.equal(called, false);
});
test("Composite rollback requires every response and independent absence checks", async (t) => {
  const path = await setup(t);
  const checked: string[] = [];
  await runCompositeFixtureAttempt(
    path,
    intents,
    [...expected, { referenceId: "p", method: "PATCH" }],
    "rollback",
    async () => ({
      status: 200,
      body: {
        compositeResponse: [
          rollback("a"),
          rollback("c"),
          rollback("p", "INVALID_FIELD"),
        ],
      },
    }),
    async (e) => {
      checked.push(e.operationId);
    },
  );
  assert.deepEqual(checked, ["a", "c"]);
  assert.equal((await cleanupFixtures(context(path))).unknownIntents, 0);
});
test("Composite ambiguous outcomes remain unresolved and never report cleanup success", async (t) => {
  const responses: (Response | Error)[] = [
    Error("PRIVATE_MARKER"),
    { status: 500, body: {} },
    { status: 200, body: {} },
    { status: 200, body: { compositeResponse: [success("a", aid)] } },
    {
      status: 200,
      body: { compositeResponse: [success("a", aid), success("a", cid)] },
    },
    {
      status: 200,
      body: { compositeResponse: [success("a", "bad"), success("c", cid)] },
    },
  ];
  for (const response of responses) {
    const path = await setup(t);
    await assert.rejects(
      runCompositeFixtureAttempt(
        path,
        intents,
        expected,
        "commit",
        async () => {
          if (response instanceof Error) throw response;
          return response;
        },
        async () => {},
      ),
      /^Error: (FIXTURES|RESULT)$/,
    );
    const cleanup = await cleanupFixtures(context(path), async () => ({
      status: 404,
      body: null,
    }));
    assert.ok(cleanup.unknownIntents > 0);
    assert.equal(cleanup.verified, false);
  }
});
test("Composite records attributable success before rejecting a malformed neighbor", async (t) => {
  const path = await setup(t);
  await assert.rejects(
    runCompositeFixtureAttempt(
      path,
      intents,
      expected,
      "commit",
      async () => ({
        status: 200,
        body: { compositeResponse: [success("a", aid), { referenceId: "c" }] },
      }),
      async () => {},
    ),
  );
  assert.equal((await events(path)).find((e) => e.kind === "known")?.id, aid);
});
test("Composite missing non-create rollback response is not complete", async (t) => {
  const path = await setup(t);
  let reconciled = false;
  await assert.rejects(
    runCompositeFixtureAttempt(
      path,
      intents,
      [...expected, { referenceId: "p", method: "PATCH" }],
      "rollback",
      async () => ({
        status: 200,
        body: {
          compositeResponse: [rollback("a"), rollback("c", "INVALID_FIELD")],
        },
      }),
      async () => {
        reconciled = true;
      },
    ),
  );
  assert.equal(reconciled, false);
});
test("Composite duplicate recovery identities retain ambiguous original while cleaning discovered IDs", async (t) => {
  const path = await setup(t);
  await assert.rejects(
    runCompositeFixtureAttempt(
      path,
      [intents[0]],
      [expected[0]],
      "rollback",
      async () => ({
        status: 200,
        body: { compositeResponse: [rollback("a", "INVALID_FIELD")] },
      }),
      async (e) => {
        for (const id of [aid, "001000000000001AAA"]) {
          const recovery = {
            ...e,
            operationId: e.operationId + "__recovered_" + id,
          };
          await recordFixtureEvent(path, recovery);
          await recordFixtureEvent(path, { ...recovery, kind: "known", id });
        }
        throw Error("FIXTURES");
      },
    ),
  );
  const cleaned = await cleanupFixtures(context(path), async () => ({
    status: 404,
    body: null,
  }));
  assert.equal(cleaned.records, 2);
  assert.equal(cleaned.unknownIntents, 1);
  assert.equal(cleaned.verified, false);
});

test("Composite expected prevalidation rejection resolves only after verified absence", async (t) => {
  const path = await setup(t);
  let checked = 0;
  await runCompositeFixtureAttempt(
    path,
    [intents[0]],
    [expected[0]],
    "prevalidation",
    async () => ({
      status: 400,
      body: [
        {
          errorCode: "INVALID_COMPOSITE_REQUEST",
          message: "Invalid or unsupported Composite request",
        },
      ],
    }),
    async () => {
      checked++;
    },
  );
  assert.equal(checked, 1);
  assert.deepEqual(
    (await events(path)).map((e) => e.kind),
    ["intent", "rejected"],
  );
});
test("Composite metadata mismatch prevents dispatch", async (t) => {
  const path = await setup(t);
  let dispatched = false;
  await assert.rejects(
    runCompositeFixtureAttempt(
      path,
      intents,
      [{ referenceId: "a", method: "POST" }],
      "commit",
      async () => {
        dispatched = true;
        return { status: 200, body: {} };
      },
      async () => {},
    ),
  );
  assert.equal(dispatched, false);
  assert.equal((await events(path)).length, 0);
});
