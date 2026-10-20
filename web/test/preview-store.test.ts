import test from "node:test";
import assert from "node:assert/strict";
import { createPinia, setActivePinia } from "pinia";
import { usePreviewStore } from "../src/stores/preview.ts";
test("preview stores isolate fixtures and reject invalid references without mutation", () => {
  setActivePinia(createPinia());
  const first = usePreviewStore();
  const before = JSON.stringify(first.$state);
  assert.throws(
    () => first.save("accounts", { Name: "X" }, "unknown"),
    /notFound/,
  );
  assert.throws(
    () => first.save("contacts", { LastName: "X", AccountId: "missing" }),
    /missingAccount/,
  );
  assert.equal(JSON.stringify(first.$state), before);
  first.save("accounts", { Name: "Changed" }, "sample-account-1");
  setActivePinia(createPinia());
  const second = usePreviewStore();
  assert.equal(
    second.find("accounts", "sample-account-1")?.Name,
    "Northstar Studio",
  );
  assert.equal(second.accounts.length, 2);
});

test("reset does not reuse record identities retained in browser history", () => {
  setActivePinia(createPinia());
  const store = usePreviewStore();
  const old = store.save("accounts", { Name: "Before reset" });
  store.reset();
  const fresh = store.save("accounts", { Name: "After reset" });
  assert.notEqual(fresh, old);
  assert.equal(store.find("accounts", old), undefined);
});
