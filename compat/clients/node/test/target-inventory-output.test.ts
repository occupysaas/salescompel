import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { constants } from "node:fs";
import { join } from "node:path";
import { validateTargetInventoryFile } from "../src/target-inventory-command.ts";
import { withDir, writeInput } from "./helpers/target-inventory-command.ts";

for (const operation of ["writeFile", "sync"] as const)
  test(`output ${operation} failure removes only the newly owned output`, async (t) =>
    withDir(async (d) => {
      const p = await writeInput(d),
        out = join(d, "out.json"),
        originalOpen = fs.open;
      t.mock.method(fs, "open", async (...args: Parameters<typeof fs.open>) => {
        const handle = await originalOpen(...args);
        if (
          !String(args[0]).endsWith("/out.json") ||
          (Number(args[1]) & constants.O_EXCL) === 0
        )
          return handle;
        return new Proxy(handle, {
          get(target, key) {
            if (key === operation)
              return async () => {
                throw new Error("INJECTED_SECRET_1234");
              };
            const value = Reflect.get(target, key);
            return typeof value === "function" ? value.bind(target) : value;
          },
        });
      });
      const r = await validateTargetInventoryFile(p, out);
      assert.equal(r.exitCode, 2);
      assert.ok(!JSON.stringify(r).includes("INJECTED_SECRET_1234"));
      await assert.rejects(fs.access(out));
      assert.ok((await fs.readFile(p)).length > 0);
    }));

test("output creation and failure cleanup stay anchored when the parent is replaced", async (t) =>
  withDir(async (d) => {
    const p = await writeInput(d),
      parent = join(d, "output"),
      outside = join(d, "private");
    await fs.mkdir(parent, { mode: 0o700 });
    await fs.mkdir(outside, { mode: 0o700 });
    const original = fs.open;
    let swapped = false;
    const opened: string[] = [];
    t.mock.method(fs, "open", async (...args: Parameters<typeof fs.open>) => {
      if ((Number(args[1]) & constants.O_EXCL) !== 0 && !swapped) {
        swapped = true;
        await fs.rename(parent, parent + "-old");
        await fs.symlink(outside, parent);
      }
      const h = await original(...args);
      opened.push(await fs.realpath(`/proc/self/fd/${h.fd}`));
      return h;
    });
    await validateTargetInventoryFile(p, join(parent, "report.json"));
    assert.ok(swapped);
    assert.ok(!opened.includes(join(outside, "report.json")));
    await assert.rejects(fs.access(join(outside, "report.json")));
  }));
