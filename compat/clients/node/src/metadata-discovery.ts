import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { createHash } from "node:crypto";
import { privateDirectory } from "./metadata-files.ts";
import { wire } from "./wire.ts";
import { readWithClient } from "./metadata-read-process.ts";
import {
  parseDiscoveryConfig,
  newInventory,
  addFact,
  addGap,
  finishInventory,
  FIELDS,
  MEMBERS,
  type DiscoveryConfig,
  type Inventory,
  type Persona,
  type Area,
} from "./metadata-inventory.ts";
export interface Identity {
  user: string;
  org: string;
}
export interface DiscoveryReader {
  identity(p: Persona): Promise<Identity>;
  describe(p: Persona, object: string): Promise<any>;
  metadata(
    type: "CustomField" | "PermissionSet" | "Profile",
    name: string,
  ): Promise<any>;
}
export function discoveryReader(
  c: DiscoveryConfig,
  env: NodeJS.ProcessEnv,
): DiscoveryReader {
  const tokens = {
    administrator: env.COMPAT_REFERENCE_TOKEN!,
    restricted: env.COMPAT_REFERENCE_RESTRICTED_TOKEN!,
  };
  return {
    async identity(p) {
      const r = await wire(
        new URL(c.origin),
        tokens[p],
        "/services/oauth2/userinfo",
        {},
      );
      const b = r.body as any;
      if (
        r.status !== 200 ||
        typeof b?.user_id !== "string" ||
        typeof b?.organization_id !== "string" ||
        !b.user_id ||
        !b.organization_id
      )
        throw new Error("IDENTITY_UNAVAILABLE");
      return { user: b.user_id, org: b.organization_id };
    },
    describe: (p, name) =>
      readWithClient({
        origin: c.origin,
        token: tokens[p],
        kind: "describe",
        name,
      }),
    metadata: (type, name) =>
      readWithClient({
        origin: c.origin,
        token: tokens.administrator,
        kind: "metadata",
        type,
        name,
      }),
  };
}
export async function verifyIdentityPair(
  reader: DiscoveryReader,
): Promise<Record<Persona, Identity>> {
  const administrator = await reader.identity("administrator"),
    restricted = await reader.identity("restricted");
  if (
    !administrator.user ||
    !restricted.user ||
    !administrator.org ||
    administrator.org !== restricted.org ||
    administrator.user === restricted.user
  )
    throw new Error("IDENTITY_MISMATCH");
  return { administrator, restricted };
}
export function sameIdentities(
  a: Record<Persona, Identity>,
  b: Record<Persona, Identity>,
) {
  return (
    a.administrator.user === b.administrator.user &&
    a.restricted.user === b.restricted.user &&
    a.administrator.org === b.administrator.org &&
    a.restricted.org === b.restricted.org
  );
}
export function validateDiscoveryEnv(
  c: DiscoveryConfig,
  env: NodeJS.ProcessEnv,
) {
  if (
    env.COMPAT_REFERENCE_URL !== c.origin ||
    !env.COMPAT_REFERENCE_TOKEN ||
    !env.COMPAT_REFERENCE_RESTRICTED_TOKEN ||
    env.COMPAT_REFERENCE_TOKEN === env.COMPAT_REFERENCE_RESTRICTED_TOKEN
  )
    throw new Error("Discovery endpoint or credentials missing/mismatched");
}
export async function collectMetadata(
  config: DiscoveryConfig,
  env: NodeJS.ProcessEnv,
  reader?: DiscoveryReader,
): Promise<Inventory> {
  const c = parseDiscoveryConfig(config);
  validateDiscoveryEnv(c, env);
  reader ??= discoveryReader(c, env);
  const i = newInventory(c);
  const artifactDir = env.DISCOVERY_ARTIFACT_DIR
    ? await privateDirectory(env.DISCOVERY_ARTIFACT_DIR)
    : undefined;
  if (artifactDir) await mkdir(join(artifactDir, "raw"), { mode: 0o700 });
  const tokens = [
    env.COMPAT_REFERENCE_TOKEN,
    env.COMPAT_REFERENCE_RESTRICTED_TOKEN,
  ];
  let identities: Record<Persona, Identity>;
  try {
    identities = await verifyIdentityPair(reader);
    i.identityVerified.initial = true;
  } catch {
    return finishInventory(i);
  }
  async function operation(
    area: Area,
    p: Persona,
    component: string,
    read: () => Promise<any>,
    project: (data: any, id: string) => void,
  ) {
    const id = `${area}:${p}:${component}`;
    const op = {
      id,
      kind: area,
      persona: p,
      component,
      state: "observed" as "observed" | "unavailable",
    };
    i.operations.push(op);
    try {
      const data = await read();
      if (artifactDir) {
        const raw = JSON.stringify(data);
        if (Buffer.byteLength(raw) > 8 * 1024 * 1024)
          throw new Error("Read evidence bound");
        const path =
          "raw/" + createHash("sha256").update(id).digest("hex") + ".json";
        await writeFile(join(artifactDir, path), raw, {
          flag: "wx",
          mode: 0o600,
        });
        i.artifacts.push({
          path,
          bytes: Buffer.byteLength(raw),
          sha256: createHash("sha256").update(raw).digest("hex"),
          component,
        });
      }
      project(data, id);
    } catch {
      op.state = "unavailable";
      addGap(i, id, area, p, component, "*", "READ_UNAVAILABLE");
    }
  }
  for (const p of ["administrator", "restricted"] as const)
    for (const [object, fields] of Object.entries(FIELDS))
      await operation(
        "describe",
        p,
        object,
        () => reader!.describe(p, object),
        (data, id) => {
          if (data?.name !== object || !Array.isArray(data.fields))
            throw new Error("Invalid describe");
          for (const property of [
            "queryable",
            "createable",
            "updateable",
            "deletable",
          ])
            addFact(i, id, "describe", p, object, property, data[property]);
          for (const field of fields) {
            const f = data.fields.find((x: any) => x.name === field);
            const component = object + "." + field;
            if (!f) {
              addGap(i, id, "describe", p, component, "*", "FIELD_NOT_VISIBLE");
              continue;
            }
            for (const property of [
              "type",
              "length",
              "nillable",
              "externalId",
              "referenceTo",
              "createable",
              "updateable",
            ])
              addFact(i, id, "describe", p, component, property, f[property]);
          }
        },
      );
  const groups = {
    CustomField: MEMBERS.CustomField,
    PermissionSet: [
      ...new Set([...MEMBERS.PermissionSet, ...c.permissionSets]),
    ],
    Profile: c.profiles,
  };
  for (const type of ["CustomField", "PermissionSet", "Profile"] as const)
    for (const name of groups[type])
      await operation(
        "metadata",
        "administrator",
        `${type}:${name}`,
        () => reader!.metadata(type, name),
        (data, id) => projectMetadata(i, type, name, data, id),
      );
  for (const [type, names] of [
    ["Profile", c.profiles],
    ["PermissionSet", c.permissionSets],
  ] as const)
    if (!names.length) {
      const id = `metadata:administrator:optional-${type}`;
      i.operations.push({
        id,
        kind: "metadata",
        persona: "administrator",
        component: `optional-${type}`,
        state: "observed",
      });
      addGap(
        i,
        id,
        "metadata",
        "administrator",
        `optional-${type}`,
        "*",
        "SCOPE_NOT_REQUESTED",
        false,
        "unrequested",
      );
    }
  try {
    i.identityVerified.final =
      sameIdentities(identities, await verifyIdentityPair(reader)) &&
      tokens[0] === env.COMPAT_REFERENCE_TOKEN &&
      tokens[1] === env.COMPAT_REFERENCE_RESTRICTED_TOKEN;
  } catch {
    i.identityVerified.final = false;
  }
  return finishInventory(i);
}
export function projectMetadata(
  i: Inventory,
  type: "CustomField" | "PermissionSet" | "Profile",
  name: string,
  data: any,
  id: string,
  area: Area = "metadata",
) {
  if (!data || data.fullName !== name) {
    addGap(
      i,
      id,
      area,
      "administrator",
      `${type}:${name}`,
      "*",
      "COMPONENT_UNAVAILABLE",
    );
    return;
  }
  const add = (component: string, property: string, value: unknown) =>
    addFact(i, id, area, "administrator", component, property, value);
  if (type === "CustomField") {
    for (const property of [
      "type",
      "length",
      "required",
      "externalId",
      "unique",
    ])
      add(name, property, data[property]);
    if (data.caseSensitive !== undefined)
      add(name, "caseSensitive", data.caseSensitive);
    return;
  }
  for (const [group, key, properties] of [
    ["fieldPermissions", "field", ["readable", "editable"]],
    [
      "objectPermissions",
      "object",
      [
        "allowRead",
        "allowCreate",
        "allowEdit",
        "allowDelete",
        "viewAllRecords",
        "modifyAllRecords",
      ],
    ],
  ] as const) {
    const entries = data[group];
    const selected =
      group === "fieldPermissions"
        ? Object.entries(FIELDS).flatMap(([o, fs]) =>
            fs.map((f) => o + "." + f),
          )
        : Object.keys(FIELDS);
    const rows =
      entries === undefined ? [] : Array.isArray(entries) ? entries : [entries];
    for (const component of selected) {
      const row = rows.find((x: any) => x?.[key] === component);
      if (!row) {
        addGap(
          i,
          id,
          area,
          "administrator",
          `${type}:${name}:${component}`,
          group,
          "DECLARATION_UNAVAILABLE",
        );
        continue;
      }
      for (const property of properties)
        add(`${type}:${name}:${component}`, property, row[property]);
    }
  }
}
