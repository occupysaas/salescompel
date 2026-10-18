// Private child process: allowlisted public read operations only, killed by parent on bounds.
import { Connection } from "jsforce";
import {
  validateReadRequest,
  type ReadRequest,
} from "./metadata-read-process.ts";
try {
  let input = "";
  for await (const chunk of process.stdin) {
    input += chunk;
    if (input.length > 65536) throw new Error("Input bound");
  }
  const r = JSON.parse(input) as ReadRequest;
  validateReadRequest(r);
  const conn = new Connection({
    instanceUrl: r.origin,
    accessToken: r.token,
    version: "65.0",
    httpProxy: undefined,
  });
  const result =
    r.kind === "describe"
      ? await conn.sobject(r.name).describe()
      : await conn.metadata.read(r.type!, r.name);
  process.stdout.write(JSON.stringify(result));
} catch {
  process.exitCode = 2;
}
