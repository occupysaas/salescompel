import { Connection } from "jsforce";
let input = "";
for await (const chunk of process.stdin) {
  input += chunk;
  if (input.length > 65536) process.exit(2);
}
try {
  const r = JSON.parse(input);
  const u = new URL(r.origin);
  if (
    u.protocol !== "https:" ||
    u.origin !== r.origin ||
    typeof r.token !== "string" ||
    typeof r.key !== "string" ||
    r.key.length > 255 ||
    !/^p9[a-f0-9]{24}-native$/.test(r.marker)
  )
    throw new Error();
  const conn = new Connection({
    instanceUrl: r.origin,
    accessToken: r.token,
    version: "65.0",
    httpProxy: undefined,
  });
  const result = await conn
    .sobject("Account")
    .upsert({ Name: r.marker, Compat_Key__c: r.key }, "Compat_Key__c");
  process.stdout.write(JSON.stringify(result));
} catch {
  process.exitCode = 2;
}
