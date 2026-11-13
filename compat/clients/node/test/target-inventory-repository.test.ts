import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { join } from "node:path";
import { makeInventory } from "./target-inventory-fixtures.ts";
import { checkInventoryRepositoryRefs } from "../src/target-inventory-command.ts";
import { withDir } from "./helpers/target-inventory-command.ts";
test("repository resolver allows bounded approved files and exact contract/profile/heading IDs only", async () =>
  withDir(async (d) => {
    await fs.mkdir(join(d, "docs/compat"), { recursive: true });
    await fs.mkdir(join(d, "locales"));
    await fs.writeFile(
      join(d, "docs/compat/contracts.yaml"),
      "contracts:\n  - id: crud\n",
    );
    await fs.writeFile(join(d, "docs/compat/profile.yaml"), "id: pilot\n");
    await fs.writeFile(join(d, "docs/compat/guide.md"), "# Example heading\n");
    await fs.writeFile(join(d, "locales/catalog.json"), "{}");
    const good = [
      { path: "docs/compat/contracts.yaml", id: "crud" },
      { path: "docs/compat/profile.yaml", id: "pilot" },
      { path: "docs/compat/guide.md", id: "example-heading" },
      { path: "locales/catalog.json" },
    ];
    assert.deepEqual(await checkInventoryRepositoryRefs(good, d), []);
    for (const ref of [
      { path: "docs/compat/profile.yaml", id: "absent" },
      { path: "docs/compat/missing" },
      { path: "locales/catalog.json", id: "absent" },
    ])
      assert.ok((await checkInventoryRepositoryRefs([ref], d)).length);
  }));
test("repository resolver rejects traversal, schemes and symlink components without opening their targets", async () =>
  withDir(async (d) => {
    await fs.mkdir(join(d, "docs/compat"), { recursive: true });
    for (const path of [
      "/etc/passwd",
      "docs/compat/../../secret",
      "https://example.invalid",
      "C:/secret",
      "docs/compat\\secret",
      "docs/compat/%2e%2e/secret",
      "docs/compat/./secret",
      "docs/compat/\0secret",
    ])
      assert.ok(
        (await checkInventoryRepositoryRefs([{ path }], d)).length,
        path,
      );
    await fs.symlink("/etc/passwd", join(d, "docs/compat/link"));
    assert.ok(
      (await checkInventoryRepositoryRefs([{ path: "docs/compat/link" }], d))
        .length,
    );
    await fs.symlink("/tmp", join(d, "docs/compat/escape"));
    assert.ok(
      (
        await checkInventoryRepositoryRefs(
          [{ path: "docs/compat/escape/private" }],
          d,
        )
      ).length,
    );
    const original = fs.open;
    const calls: string[] = [];
    // Real resolver must reject the symlink before calling open on it.
    const mock = (() => {
      const saved = fs.open;
      (fs as Record<string, unknown>).open = async (...args: unknown[]) => {
        calls.push(String(args[0]));
        return (saved as (...a: unknown[]) => Promise<unknown>)(...args);
      };
      return () => {
        fs.open = saved;
      };
    })();
    try {
      await checkInventoryRepositoryRefs([{ path: "docs/compat/link" }], d);
      assert.equal(calls.length, 0);
    } finally {
      mock();
      fs.open = original;
    }
  }));

import {
  validateTargetInventoryRepository,
  validateRepository,
} from "../src/validate.ts";
import { ROOT } from "../src/versions.ts";

test("repository inventory integration rejects malformed examples/schema, real classification and broken IDs", async () =>
  withDir(async (d) => {
    await fs.mkdir(join(d, "compat/schemas"), { recursive: true });
    await fs.mkdir(join(d, "docs/compat/inventories"), { recursive: true });
    const schemaPath = join(d, "compat/schemas/target-inventory.schema.json"),
      samplePath = join(
        d,
        "docs/compat/inventories/synthetic-account-contact.yaml",
      );
    const originalSchema = await fs.readFile(
      join(ROOT, "compat/schemas/target-inventory.schema.json"),
    );
    await fs.writeFile(schemaPath, originalSchema);
    const v = makeInventory();
    v.sources[0].locator = "https://127.0.0.1:1/never-fetch";
    v.sections.dataMetadata.items[0].repositoryRefs = [
      { path: "docs/compat/contracts.yaml", id: "crud" },
    ];
    await fs.writeFile(
      join(d, "docs/compat/contracts.yaml"),
      "contracts:\n - id: crud\n",
    );
    const write = () => fs.writeFile(samplePath, JSON.stringify(v));
    await write();
    assert.deepEqual(await validateTargetInventoryRepository(d), []);
    const before = structuredClone(v);
    assert.deepEqual(JSON.parse(await fs.readFile(samplePath, "utf8")), before);
    await fs.writeFile(samplePath, "invalid: [INJECTED_SECRET_1234");
    let issues = await validateTargetInventoryRepository(d);
    assert.ok(issues.length);
    assert.ok(!issues.join("").includes("INJECTED_SECRET_1234"));
    await write();
    await fs.writeFile(schemaPath, '{"type":"nonsense"}');
    assert.ok((await validateTargetInventoryRepository(d)).length);
    await fs.writeFile(schemaPath, originalSchema);
    v.classification = "real";
    await write();
    assert.ok((await validateTargetInventoryRepository(d)).length);
    v.classification = "synthetic";
    v.sections.dataMetadata.items[0].repositoryRefs[0].id = "missing";
    await write();
    assert.ok((await validateTargetInventoryRepository(d)).length);
    v.sections.dataMetadata.items[0].repositoryRefs[0].id = "crud";
    await write();
    await fs.unlink(join(d, "docs/compat/contracts.yaml"));
    assert.ok((await validateTargetInventoryRepository(d)).length);
  }));
test("ordinary repository validation accepts the checked-in incomplete synthetic example", async () => {
  const issues = await validateRepository();
  assert.deepEqual(issues, []);
});

test("repository validation refuses symlinked schema/example files before reading them", async () =>
  withDir(async (d) => {
    const root = join(d, "repo");
    await fs.mkdir(join(root, "compat/schemas"), { recursive: true });
    await fs.mkdir(join(root, "docs/compat/inventories"), { recursive: true });
    const schema = join(root, "compat/schemas/target-inventory.schema.json"),
      sample = join(
        root,
        "docs/compat/inventories/synthetic-account-contact.yaml",
      );
    await fs.copyFile(
      join(ROOT, "compat/schemas/target-inventory.schema.json"),
      schema,
    );
    const external = join(d, "external.yaml");
    await fs.writeFile(external, JSON.stringify(makeInventory()));
    await fs.symlink(external, sample);
    assert.ok((await validateTargetInventoryRepository(root)).length);
    await fs.unlink(sample);
    await fs.writeFile(sample, JSON.stringify(makeInventory()));
    await fs.rename(schema, join(d, "external-schema.json"));
    await fs.symlink(join(d, "external-schema.json"), schema);
    assert.ok((await validateTargetInventoryRepository(root)).length);
  }));

test("ordinary validation fails closed when the inventory example cannot be opened", async (t) => {
  const original = fs.open;
  t.mock.method(fs, "open", async (...args: Parameters<typeof fs.open>) => {
    if (String(args[0]).endsWith("/synthetic-account-contact.yaml"))
      throw new Error("INJECTED_SECRET_1234");
    return original(...args);
  });
  const issues = await validateRepository();
  assert.ok(issues.some((i) => i.startsWith("Target inventory:")));
  assert.ok(!issues.join("").includes("INJECTED_SECRET_1234"));
});

test("replacing a checked read directory with an external symlink cannot open private files", async (t) =>
  withDir(async (d) => {
    const root = join(d, "repo"),
      parent = join(root, "docs/compat"),
      outside = join(d, "private");
    await fs.mkdir(parent, { recursive: true });
    await fs.mkdir(outside);
    await fs.writeFile(join(parent, "target.yaml"), "id: public\n");
    await fs.writeFile(join(outside, "target.yaml"), "id: private\n");
    const originalLstat = fs.lstat,
      originalOpen = fs.open;
    let swapped = false;
    const opened: string[] = [];
    t.mock.method(fs, "lstat", async (...args: Parameters<typeof fs.lstat>) => {
      const info = await originalLstat(...args);
      if (args[0] === join(parent, "target.yaml") && !swapped) {
        swapped = true;
        await fs.rename(parent, parent + "-old");
        await fs.symlink(outside, parent);
      }
      return info;
    });
    t.mock.method(fs, "open", async (...args: Parameters<typeof fs.open>) => {
      const h = await originalOpen(...args);
      opened.push(await fs.realpath(`/proc/self/fd/${h.fd}`));
      return h;
    });
    const issues = await checkInventoryRepositoryRefs(
      [{ path: "docs/compat/target.yaml", id: "private" }],
      root,
    );
    assert.ok(issues.length);
    assert.ok(!opened.includes(join(outside, "target.yaml")));
  }));
test("a directory swap just before final open keeps the read anchored to the checked parent", async (t) =>
  withDir(async (d) => {
    const root = join(d, "repo"),
      parent = join(root, "docs/compat"),
      outside = join(d, "private");
    await fs.mkdir(parent, { recursive: true });
    await fs.mkdir(outside);
    await fs.writeFile(join(parent, "target.yaml"), "id: public\n");
    await fs.writeFile(join(outside, "target.yaml"), "id: private\n");
    const original = fs.open;
    let swapped = false;
    const opened: string[] = [];
    t.mock.method(fs, "open", async (...args: Parameters<typeof fs.open>) => {
      if (String(args[0]).endsWith("/target.yaml") && !swapped) {
        swapped = true;
        await fs.rename(parent, parent + "-old");
        await fs.symlink(outside, parent);
      }
      const h = await original(...args);
      opened.push(await fs.realpath(`/proc/self/fd/${h.fd}`));
      return h;
    });
    await checkInventoryRepositoryRefs(
      [{ path: "docs/compat/target.yaml", id: "private" }],
      root,
    );
    assert.ok(swapped);
    assert.ok(!opened.includes(join(outside, "target.yaml")));
  }));
test("referenced YAML collection keys cannot emit process warnings", async (t) =>
  withDir(async (d) => {
    await fs.mkdir(join(d, "docs/compat"), { recursive: true });
    await fs.writeFile(
      join(d, "docs/compat/private.yaml"),
      "? [INJECTED_SECRET_1234]\n: x\n",
    );
    const warnings: string[] = [];
    t.mock.method(process, "emitWarning", (message: unknown) => {
      warnings.push(String(message));
    });
    const issues = await checkInventoryRepositoryRefs(
      [{ path: "docs/compat/private.yaml", id: "expected" }],
      d,
    );
    assert.ok(issues.length);
    assert.equal(warnings.length, 0);
  }));
