import { test as base, expect, type APIRequestContext } from "@playwright/test";
import { randomBytes } from "node:crypto";
import {
  recordFixtureEvent,
  type FixtureEvent,
} from "../../../compat/clients/node/src/candidate-acceptance-fixtures.ts";
export async function createBrowserFixture(
  request: APIRequestContext,
  object: "Account" | "Contact",
  data: Record<string, string>,
  names?: string[],
) {
  const name = data[object === "Account" ? "Name" : "LastName"];
  const e: FixtureEvent = {
    schemaVersion: 1,
    runId: process.env.CANDIDATE_RUN_ID!,
    operationId: "browser-api-" + randomBytes(8).toString("hex"),
    principal: "admin",
    object,
    names: names ?? [name, name + " edited", name + " updated"],
    kind: "intent",
  };
  if (process.env.CANDIDATE_FIXTURE_JOURNAL)
    await recordFixtureEvent(process.env.CANDIDATE_FIXTURE_JOURNAL, e);
  const r = await request.post(
    `${process.env.CANDIDATE_URL}/services/data/v65.0/sobjects/${object}`,
    {
      headers: { Authorization: `Bearer ${process.env.CANDIDATE_TOKEN}` },
      data,
    },
  );
  expect(r.status()).toBe(201);
  const value = await r.json();
  if (process.env.CANDIDATE_FIXTURE_JOURNAL)
    await recordFixtureEvent(process.env.CANDIDATE_FIXTURE_JOURNAL, {
      ...e,
      kind: "known",
      id: value.id,
    });
  return r;
}
export const test = base.extend<{ fixtureJournal: void }>({
  fixtureJournal: [
    async ({ page }, use) => {
      const events = new Map<object, FixtureEvent>(),
        pending: Promise<unknown>[] = [];
      const path = process.env.CANDIDATE_FIXTURE_JOURNAL;
      if (path) {
        await page.route("**/services/data/v65.0/sobjects/*", async (route) => {
          const req = route.request(),
            object = req.url().split("/").at(-1);
          if (
            req.method() === "POST" &&
            (object === "Account" || object === "Contact")
          ) {
            const data = req.postDataJSON();
            const name = data[object === "Account" ? "Name" : "LastName"];
            const e: FixtureEvent = {
              schemaVersion: 1,
              runId: process.env.CANDIDATE_RUN_ID!,
              operationId: "browser-page-" + randomBytes(8).toString("hex"),
              principal: (Object.entries({
                admin: process.env.CANDIDATE_TOKEN,
                restricted: process.env.CANDIDATE_RESTRICTED_TOKEN,
                other: process.env.CANDIDATE_OTHER_TOKEN,
              }).find(
                ([, token]) =>
                  req.headers().authorization === `Bearer ${token}`,
              )?.[0] ?? "invalid") as FixtureEvent["principal"],
              object,
              names: [name, name + " edited", name + " updated"],
              kind: "intent",
            };
            await recordFixtureEvent(path, e);
            events.set(req, e);
          }
          await route.continue();
        });
        page.on("response", (r) => {
          const e = events.get(r.request());
          if (e)
            pending.push(
              (async () => {
                const body = await r.json();
                if (r.status() !== 201 || !body.id)
                  throw new Error("fixture create unavailable");
                await recordFixtureEvent(path, {
                  ...e,
                  kind: "known",
                  id: body.id,
                });
              })(),
            );
        });
      }
      await use();
      await Promise.all(pending);
    },
    { auto: true },
  ],
});
