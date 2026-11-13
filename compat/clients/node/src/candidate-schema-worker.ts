import { readFile, lstat } from "node:fs/promises";
import { join, resolve } from "node:path";
import {
  runSchemaClientGate,
  type SchemaClientContext,
} from "./candidate-schema.ts";
try {
  const path = process.argv[2];
  const info = await lstat(path);
  if (!info.isFile() || info.isSymbolicLink() || info.mode & 0o077)
    throw new Error("SOURCE");
  const c = JSON.parse(await readFile(path, "utf8")) as SchemaClientContext;
  const url = new URL(c.url);
  if (
    !/^[a-f0-9]{24}$/.test(c.runId) ||
    url.protocol !== "https:" ||
    !["localhost", "127.0.0.1"].includes(url.hostname) ||
    resolve(path) !== join(c.scratch, "schema-client-context.json") ||
    c.fixtureJournal !== join(c.scratch, "fixtures.jsonl")
  )
    throw new Error("SOURCE");
  const result = await runSchemaClientGate(c);
  process.stdout.write(JSON.stringify(result) + "\n");
} catch (e: unknown) {
  const err = e as { code?: string };
  const code =
    typeof err.code === "string" && /^[A-Za-z0-9_-]{1,80}$/.test(err.code)
      ? err.code
      : "ASSERTION";
  process.stderr.write("Schema client failure: " + code + "\n");
  process.exitCode = 1;
}
