import { open, readFile, unlink } from "node:fs/promises";
import { setTimeout as delay } from "node:timers/promises";
import {
  recordFixtureEvent,
  type FixtureEvent,
  type Response,
} from "./candidate-acceptance-fixtures.ts";
type Expected = { referenceId: string; method: "POST" | "PATCH" | "DELETE" };
type CompositeEntry = {
  referenceId?: string;
  httpStatusCode: number;
  httpHeaders?: unknown;
  body?: unknown;
};
const idPattern = /^[A-Za-z0-9]{18}$/;
const isObject = (v: unknown): v is Record<string, any> =>
  v !== null && typeof v === "object" && !Array.isArray(v);
const errorBody = (
  body: unknown,
): body is Array<{ errorCode: string; message: string }> =>
  Array.isArray(body) &&
  body.length === 1 &&
  isObject(body[0]) &&
  typeof body[0].errorCode === "string" &&
  typeof body[0].message === "string";
const emptyHeaders = (v: unknown) => isObject(v) && Object.keys(v).length === 0;
export async function runCompositeFixtureAttempt(
  journalPath: string,
  intents: FixtureEvent[],
  expected: readonly Expected[],
  outcome: "commit" | "rollback" | "prevalidation",
  perform: () => Promise<Response>,
  reconcileAbsent: (intent: FixtureEvent) => Promise<void>,
): Promise<Response> {
  try {
    const refs = new Set(expected.map((e) => e.referenceId));
    const posts = expected.filter((e) => e.method === "POST");
    if (
      refs.size !== expected.length ||
      expected.length < 1 ||
      expected.length > 25 ||
      intents.length !== posts.length ||
      new Set(intents.map((e) => e.operationId)).size !== intents.length ||
      intents.some(
        (e) =>
          e.kind !== "intent" ||
          !posts.some((p) => p.referenceId === e.operationId),
      )
    )
      throw Error("FIXTURES");
    await reserveCompositeIntents(journalPath, intents);
    const response = await perform();
    const entries = (response.body as { compositeResponse?: unknown })
      .compositeResponse;
    // Resolve only independently attributable create IDs, before validating neighbors.
    if (response.status === 200 && Array.isArray(entries)) {
      const list = entries as CompositeEntry[];
      for (const intent of intents) {
        const matches = list.filter(
          (e) => e?.referenceId === intent.operationId,
        );
        if (matches.length === 1) {
          const e = matches[0];
          if (
            e.httpStatusCode === 201 &&
            isObject(e.body) &&
            e.body.success === true &&
            Array.isArray(e.body.errors) &&
            e.body.errors.length === 0 &&
            typeof e.body.id === "string" &&
            idPattern.test(e.body.id)
          )
            await recordFixtureEvent(journalPath, {
              ...intent,
              kind: "known",
              id: e.body.id,
            });
        }
      }
    }
    if (outcome === "prevalidation") {
      if (
        response.status !== 400 ||
        !errorBody(response.body) ||
        !["INVALID_COMPOSITE_REQUEST", "JSON_PARSER_ERROR"].includes(
          response.body[0].errorCode,
        )
      )
        throw Error("RESULT");
    } else {
      if (
        response.status !== 200 ||
        !isObject(response.body) ||
        Object.keys(response.body).length !== 1 ||
        !Array.isArray(entries) ||
        entries.length !== expected.length
      )
        throw Error("RESULT");
      let originalErrors = 0;
      const list = entries as CompositeEntry[];
      for (let i = 0; i < expected.length; i++) {
        const e = list[i];
        if (
          !isObject(e) ||
          Object.keys(e).sort().join(",") !==
            "body,httpHeaders,httpStatusCode,referenceId" ||
          e.referenceId !== expected[i].referenceId ||
          !emptyHeaders(e.httpHeaders)
        )
          throw Error("RESULT");
        if (outcome === "commit") {
          if (expected[i].method === "POST") {
            if (
              e.httpStatusCode !== 201 ||
              !isObject(e.body) ||
              Object.keys(e.body).sort().join(",") !== "errors,id,success" ||
              e.body.success !== true ||
              !Array.isArray(e.body.errors) ||
              e.body.errors.length !== 0 ||
              typeof e.body.id !== "string" ||
              !idPattern.test(e.body.id)
            )
              throw Error("RESULT");
          } else if (e.httpStatusCode !== 204 || e.body !== null)
            throw Error("RESULT");
        } else {
          if (![400, 404].includes(e.httpStatusCode) || !errorBody(e.body))
            throw Error("RESULT");
          if (e.body[0].errorCode !== "PROCESSING_HALTED") originalErrors++;
          else if (e.httpStatusCode !== 400) throw Error("RESULT");
        }
      }
      if (outcome === "rollback" && originalErrors !== 1) throw Error("RESULT");
    }
    if (outcome !== "commit") {
      // Do not resolve any intent as rejected until every absence check succeeds.
      for (const intent of intents) await reconcileAbsent(intent);
      for (const intent of intents)
        await recordFixtureEvent(journalPath, { ...intent, kind: "rejected" });
    }
    return response;
  } catch {
    throw Error("FIXTURES");
  }
}

// Reserve create identities before dispatch. A short sidecar lock serializes the
// read/check/append across the independent owned Node test processes. A stale
// lock fails closed rather than guessing whether another writer is alive.
async function reserveCompositeIntents(
  path: string,
  intents: FixtureEvent[],
): Promise<void> {
  if (intents.length === 0) return;
  const lockPath = path + ".composite-lock",
    deadline = Date.now() + 5000;
  let lock: Awaited<ReturnType<typeof open>> | undefined;
  while (!lock) {
    try {
      lock = await open(lockPath, "wx", 0o600);
    } catch (error: unknown) {
      const err = error as { code?: string } | null | undefined;
      if (err?.code !== "EEXIST" || Date.now() >= deadline)
        throw Error("FIXTURES");
      await delay(10);
    }
  }
  try {
    const text = await readFile(path, "utf8");
    if (text && !text.endsWith("\n")) throw Error("FIXTURES");
    const used = new Set<string>();
    for (const line of text.split("\n").filter(Boolean)) {
      const event = JSON.parse(line) as FixtureEvent;
      if (
        !event ||
        event.runId !== intents[0].runId ||
        typeof event.operationId !== "string"
      )
        throw Error("FIXTURES");
      used.add(event.operationId);
    }
    if (intents.some((e) => used.has(e.operationId))) throw Error("FIXTURES");
    for (const intent of intents) await recordFixtureEvent(path, intent);
  } finally {
    await lock.close();
    await unlink(lockPath);
  }
}
