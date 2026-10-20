import test from "node:test";
import assert from "node:assert/strict";
import { readPreference, writePreference } from "../src/preferences-core.ts";
import { latestLocaleLoader } from "../src/i18n-core.ts";

test("preferences accept valid values and survive unavailable storage", () => {
  const good = { getItem: () => "dark", setItem: () => {} };
  assert.equal(
    readPreference(good, "theme", ["system", "light", "dark"], "system"),
    "dark",
  );
  assert.equal(
    readPreference({ getItem: () => "bad" }, "theme", ["light"], "light"),
    "light",
  );
  const broken = {
    getItem: () => {
      throw Error("denied");
    },
    setItem: () => {
      throw Error("quota");
    },
  };
  assert.equal(readPreference(broken, "theme", ["system"], "system"), "system");
  assert.doesNotThrow(() => writePreference(broken, "theme", "light"));
  assert.equal(
    readPreference(undefined, "locale", ["en_US"], "en_US"),
    "en_US",
  );
});
test("only latest async locale selection commits, even when promises resolve backwards", async () => {
  const pending = new Map<string, (value: string) => void>();
  const committed: string[] = [];
  const load = latestLocaleLoader(
    (code: string) =>
      new Promise<string>((resolve) => pending.set(code, resolve)),
    (value) => committed.push(value),
  );
  const old = load("fr");
  const fresh = load("iw");
  pending.get("iw")!("he:rtl");
  await fresh;
  pending.get("fr")!("fr:ltr");
  await old;
  assert.deepEqual(committed, ["he:rtl"]);
});
