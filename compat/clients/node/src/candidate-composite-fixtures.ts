import {
  recordFixtureEvent,
  type FixtureEvent,
  type Response,
} from "./candidate-acceptance-fixtures.ts";
type Expected = { referenceId: string; method: "POST" | "PATCH" | "DELETE" };
const idPattern = /^[A-Za-z0-9]{18}$/;
const isObject = (v: any) =>
  v !== null && typeof v === "object" && !Array.isArray(v);
const errorBody = (body: any) =>
  Array.isArray(body) &&
  body.length === 1 &&
  isObject(body[0]) &&
  typeof body[0].errorCode === "string" &&
  typeof body[0].message === "string";
const emptyHeaders = (v: any) => isObject(v) && Object.keys(v).length === 0;
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
    for (const intent of intents) await recordFixtureEvent(journalPath, intent);
    const response = await perform();
    const entries = response.body?.compositeResponse;
    // Resolve only independently attributable create IDs, before validating neighbors.
    if (response.status === 200 && Array.isArray(entries)) {
      for (const intent of intents) {
        const matches = entries.filter(
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
      for (let i = 0; i < expected.length; i++) {
        const e = entries[i];
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
